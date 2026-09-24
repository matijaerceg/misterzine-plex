#!/usr/bin/env python3
"""plexplay - play one Plex library item on the PlexCRT core.

    plexplay.py "title substring" [offset_s]   search the movie library, play first hit
    plexplay.py rk:<rating-key> [offset_s]    play a ratingKey directly
    plexplay.py list [section]                 list a few titles
    plexplay.py ctl <command>                  drive a running player (see below)

Without an offset, playback resumes from the server's saved position (viewOffset)
when there is one, like every other Plex client.

Asks the server for a 720-wide H.264 transcode as progressive Matroska over
plain HTTP (one GET, one demux), pipes it through ffmpeg: video as planar YUV
and audio as PCM in one AVI into plexfb, which shows the picture from the
core's frame ring and releases the sound against the same field clock. The
frame is vertically scaled so the square-pixel picture Plex delivers has the
right shape on a 4:3 CRT with 720 non-square pixels per line, then padded to
720x480.

Control while playing: write a line to the FIFO /tmp/plexplay.ctl
    pause | resume | toggle       hold / continue (audio stops with the picture)
    seek <seconds>                absolute position
    seek +30 | seek -10           relative
    stop                          end playback
`plexplay.py ctl seek +30` does the write for you.

Robustness:
  - timeline pings every 10 s (state playing/paused/stopped) so the server keeps
    the transcoder alive, records the resume point and marks the item watched;
  - if the pipeline dies before the end (network drop, server restart) playback
    restarts from the last shown position, up to RETRIES times;
  - if ffmpeg cannot open the stream at all (its own HTTP or TLS client failing
    against a server this script reaches fine), the stream is fetched here
    and piped into ffmpeg instead; PLEX_STREAM=pipe selects that from the start;
  - stop/seek/end all shut the transcode session down; a seek leaves the
    last frame on screen until the new stream's first frame.
"""
import os, sys, uuid, subprocess, signal, threading, time, errno, socket, stat as st_
import urllib.request, urllib.parse, urllib.error
sys.stdout.reconfigure(line_buffering=True)   # logs stay readable when redirected
import xml.etree.ElementTree as ET

HOST = os.environ.get('PLEX_HOST', '').rstrip('/')
TOKEN = os.environ.get('PLEX_TOKEN', '')
if not HOST or not TOKEN:
    raise SystemExit('Sign in through MisterZine Plex Core before starting playback.')
CLIENT_ID = os.environ.get('PLEX_CLIENT_ID') or 'mister-plexcrt-0001'
BITRATE = int(os.environ.get('PLEX_BITRATE') or 3000)   # kbit/s cap for the transcode
# gain the server applies when it folds surround to stereo (100 is unity);
# stereo tracks are copied, so it changes nothing for them
AUDIO_BOOST = int(os.environ.get('PLEX_AUDIO_BOOST') or 175)
HOME = os.path.dirname(os.path.abspath(__file__))
FF = os.environ.get('PLEX_FFMPEG') or os.path.join(HOME, 'ffmpeg')
PLEXFB = os.path.join(HOME, 'plexfb')
CTL_FIFO = '/tmp/plexplay.ctl'
STATUS = '/tmp/plexfb.stat'
PLAYSTAT = '/tmp/plexplay.stat'      # for the UI's overlay: position, length, paused
TIMELINE_S = 10          # ping interval while playing or paused
RETRIES = 3              # restarts after an unexpected pipeline death
END_SLACK = 5.0          # within this many seconds of the end counts as finished

# PMS matches a hardcoded client profile from these; "Chrome" is the one that works.
PLEX_HDRS = {
    'X-Plex-Token': TOKEN,
    'X-Plex-Client-Identifier': CLIENT_ID,
    'X-Plex-Product': 'Plex Web',
    'X-Plex-Version': '4.0.0',
    'X-Plex-Platform': 'Chrome',
    'X-Plex-Platform-Version': '120.0',
    'X-Plex-Device': 'Linux',
    'X-Plex-Device-Name': 'MiSTer CRT',
    'Accept': 'application/xml',
}

def safe_text(value):
    text = str(value)
    for secret in (TOKEN, urllib.parse.quote(TOKEN, safe=''), urllib.parse.quote_plus(TOKEN)):
        if secret: text = text.replace(secret, '[redacted]')
    return text

def log(*a):
    print(time.strftime('%H:%M:%S'), *(safe_text(v) for v in a))

def drain_stderr(pipe):
    with pipe:
        while True:
            line = pipe.readline(65537)
            if not line: return
            if len(line) > 65536:
                while line and not line.endswith(b'\n'): line = pipe.readline(65537)
                log('[oversized decoder log line omitted]')
            else:
                log(line.decode('utf-8', 'replace').rstrip())

def pump(resp, sink):
    """copy a stream response into ffmpeg's stdin until either side ends"""
    try:
        with resp, sink:
            while True:
                chunk = resp.read(256 * 1024)
                if not chunk: return
                sink.write(chunk)
    except (OSError, ValueError):
        return

def get(path, params=None, timeout=30):
    url = HOST + path
    if params:
        url += ('&' if '?' in url else '?') + urllib.parse.urlencode(params)
    r = urllib.request.urlopen(urllib.request.Request(url, headers=PLEX_HDRS), timeout=timeout)
    return r.read()

def find(query, section=1):
    root = ET.fromstring(get('/library/sections/%d/all' % section, {'title': query}))
    return [(v.get('ratingKey'), v.get('title'), v.get('year'), int(v.get('duration', 0)) // 60000)
            for v in root.findall('Video')]

def item_info(rk):
    """title, duration (s), saved resume point (s)"""
    v = ET.fromstring(get('/library/metadata/%s' % rk)).find('Video')
    return (v.get('title'), int(v.get('duration', 0)) / 1000.0, int(v.get('viewOffset', 0)) / 1000.0)

def read_status():
    """plexfb's progress file -> dict, or {} if not there yet"""
    try:
        d = {}
        for kv in open(STATUS).read().split():
            k, v = kv.split('=', 1)
            d[k] = float(v)
        return d
    except (OSError, ValueError):
        return {}


class StreamError(Exception):
    """the stream could not be fetched this time; worth another try"""


class Player:
    def __init__(self, rk, offset):
        self.rk = rk
        self.offset = offset          # stream start position, seconds
        self.p_ff = self.p_fb = None
        self.resp = None              # the stream response when fetched here
        self.pump = os.environ.get('PLEX_STREAM') == 'pipe'
        self.sess = None
        self.fps = 23.976
        self.paused = False
        self.pending = None           # 'stop' | ('seek', seconds)
        self.last_pos = 0.0
        self.lock = threading.Lock()
        self.title, self.duration, _ = item_info(rk)

    # ---- position ----
    def position(self):
        s = read_status()
        return self.offset + s.get('pos', 0.0)

    def write_playstat(self):
        try:
            tmp = PLAYSTAT + '.tmp'
            with open(tmp, 'w') as f:
                f.write('{"pos": %.2f, "dur": %.2f, "paused": %s, "rk": "%s"}\n'
                        % (self.position(), self.duration, 'true' if self.paused else 'false', self.rk))
            os.replace(tmp, PLAYSTAT)
        except OSError:
            pass

    # ---- one transcode session ----
    def start(self):
        # one session id for the whole play, like plex-for-kodi: a seek or a
        # restart re-issues decision+start on it and the server replaces the
        # transcoder; stop once at the end.
        if not self.sess: self.sess = str(uuid.uuid4())
        sess = self.sess
        params = {
            'path': '/library/metadata/%s' % self.rk, 'mediaIndex': 0, 'partIndex': 0,
            'protocol': 'http', 'directPlay': 0, 'directStream': 0,
            'videoResolution': '720x480', 'maxVideoBitrate': BITRATE, 'videoQuality': 100,
            'audioBoost': AUDIO_BOOST, 'subtitles': 'burn',
            # the Plex Web profile allows six-channel AAC; cap audio at stereo so the
            # server downmixes (and boosts) rather than ffmpeg on the ARM
            'X-Plex-Client-Profile-Extra': 'add-limitation(scope=videoAudioCodec&scopeName=*&type=upperBound&name=audio.channels&value=2)',
            'session': sess, 'X-Plex-Session-Identifier': sess,
            'copyts': 1, 'offset': int(self.offset), 'fastSeek': 1,
            'location': 'wan', 'mediaBufferSize': 12288, 'hasMDE': 1,
        }
        # The server 400s a bare start request; a decision call on the same session
        # first is what makes it accept the stream request (and tells us the shape).
        root = ET.fromstring(get('/video/:/transcode/universal/decision', params, timeout=60))
        med = root.find('.//Media')
        mg = lambda k: med.get(k) if med is not None else '?'
        as_ = root.find('.//Stream[@streamType="2"]')
        log('decision: %s -> %sx%s %s/%s %sch (boost %d), cap %d kbps' % (root.get('transcodeDecisionText'),
            mg('width'), mg('height'), mg('videoCodec'), mg('audioCodec'),
            as_.get('channels') if as_ is not None else '?', AUDIO_BOOST, BITRATE))
        if root.get('transcodeDecisionCode') not in (None, '1000', '1001'):
            # the server will not transcode this file: say why and give up
            self.show_error(safe_text(root.get('transcodeDecisionText') or 'Cannot play this file.'))
            raise RuntimeError('unplayable: %s' % root.get('transcodeDecisionText'))
        # Source frame rate drives the presenter: 23.976 -> 2.5 fields per frame,
        # which the presenter turns into a 3:2 field cadence, like a DVD player.
        FR = {'24p': 23.976, 'NTSC': 29.97, 'PAL': 25.0, '25p': 25.0, '30p': 29.97,
              '50p': 50.0, '60p': 59.94, 'ntsc': 29.97, 'pal': 25.0}
        fr_tag = (med.get('videoFrameRate') if med is not None else None) or ''
        vs_ = root.find('.//Stream[@streamType="1"]')
        self.fps = FR.get(fr_tag) or (float(vs_.get('frameRate')) if vs_ is not None and vs_.get('frameRate') else 29.97)
        log('frame rate: %s -> %.3f fps (%.2f fields per frame), offset %ds' % (fr_tag or '?', self.fps, 59.94 / self.fps, self.offset))
        params.update({k:v for k,v in PLEX_HDRS.items() if k not in ('Accept','X-Plex-Token')})
        url = HOST + '/video/:/transcode/universal/start.mkv?' + urllib.parse.urlencode(params)

        # vertical scale for a 4:3 CRT with 720 px per line, then pad to the raster
        # a 4:3 source works out a few lines taller than 480: cap it, or pad fails
        vf = ('scale=720:min(480\\,trunc(480*4/3*ih/iw/2)*2):flags=fast_bilinear,'
              'pad=720:480:0:(480-ih)/2,format=yuv420p')
        # the H.264 loop filter costs a fifth of the machine and composite
        # blurs what it smooths: off unless PLEX_LOOP_FILTER asks for it
        decode = [] if os.environ.get('PLEX_LOOP_FILTER') else ['-skip_loop_filter', 'all', '-flags2', 'fast']
        output = ['-map', '0:v:0', '-vf', vf, '-c:v', 'rawvideo', '-pix_fmt', 'yuv420p',
                  '-map', '0:a:0', '-ac', '2', '-ar', '48000', '-c:a', 'pcm_s16le',
                  '-f', 'avi', 'pipe:1']
        if self.pump:
            self.resp = self.open_stream(url)
            ff = [FF, '-loglevel', 'warning', '-threads', '2'] + decode + ['-i', 'pipe:0'] + output
        else:
            ff = [FF, '-nostdin', '-loglevel', 'warning', '-threads', '2'] + decode + [
                  '-reconnect', '1', '-reconnect_streamed', '1', '-rw_timeout', '15000000',
                  '-headers', 'X-Plex-Token: '+TOKEN+'\r\n', '-i', url] + output
        fb = [PLEXFB, 'avi', '%.4f' % self.fps]
        env = dict(os.environ, PLEXFB_DEV='/dev/fb0', PLEXFB_STATUS=STATUS)
        try: os.unlink(STATUS)
        except OSError: pass
        self.p_ff = subprocess.Popen(ff, stdin=subprocess.PIPE if self.pump else None,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env)
        threading.Thread(target=drain_stderr, args=(self.p_ff.stderr,), daemon=True).start()
        if self.pump:
            threading.Thread(target=pump, args=(self.resp, self.p_ff.stdin), daemon=True).start()
        self.p_fb = subprocess.Popen(fb, stdin=self.p_ff.stdout, env=env)
        self.p_ff.stdout.close()
        self.paused = False

    def open_stream(self, url):
        # The same client that just made the decision call fetches the stream,
        # so a server answer is logged as such, not as a decoder I/O error.
        try:
            return urllib.request.urlopen(urllib.request.Request(url, headers=PLEX_HDRS), timeout=60)
        except urllib.error.HTTPError as e:
            body = ''
            try: body = e.read(300).decode('utf-8', 'replace').replace('\n', ' ').strip()
            except Exception: pass
            try: e.close()
            except Exception: pass
            log('stream request: HTTP %d %s: %s' % (e.code, e.reason, body))
            self.show_error('The server refused the stream (HTTP %d).' % e.code)
            raise RuntimeError('unplayable: the server refused the stream (HTTP %d)' % e.code)
        except (urllib.error.URLError, OSError) as e:
            # a network hiccup, not a verdict: the main loop retries like any
            # other early pipeline death
            log('stream request: %r' % e)
            raise StreamError(type(e).__name__)

    def show_error(self, text):
        try:
            with open(PLAYSTAT + '.err', 'w') as f: f.write(text)
        except OSError: pass

    def kill(self):
        # ffmpeg first, so it is gone before its pipe closes and does not log
        # a screenful of broken-pipe errors; plexfb blanks the screen on SIGTERM
        for p in (self.p_ff, self.p_fb):
            if p and p.poll() is None:
                try: p.terminate()
                except Exception: pass
            if p:
                try: p.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    p.kill(); p.wait()
        if self.resp:
            # a pump thread blocked in a read must not hold up a stop or seek:
            # shut the socket down under it before closing the response
            try: self.resp.fp.raw._sock.shutdown(socket.SHUT_RDWR)
            except Exception: pass
            try: self.resp.close()
            except Exception: pass
            self.resp = None

    def end_session(self):
        # the transcoder was reused across seeks and restarts: shut it once.
        # 404 means the server already tore it down when the socket closed.
        if not self.sess: return
        try: get('/video/:/transcode/universal/stop', {'session': self.sess}, timeout=10)
        except urllib.error.HTTPError as e:
            if e.code != 404: log('stop session: %r' % e)
        except Exception as e: log('stop session: %r' % e)
        self.sess = None

    # ---- timeline ----
    def timeline(self, state):
        try:
            get('/:/timeline', {
                'ratingKey': self.rk, 'key': '/library/metadata/%s' % self.rk,
                'state': state, 'time': int(self.position() * 1000),
                'duration': int(self.duration * 1000),
                'X-Plex-Session-Identifier': self.sess or '',
                'identifier': 'com.plexapp.plugins.library', 'hasMDE': 1, 'continuing': 0,
            }, timeout=10)
        except Exception as e:
            log('timeline %s: %r' % (state, e))

    # ---- control ----
    def command(self, line):
        w = line.split()
        if not w: return
        c = w[0]
        if c in ('pause', 'resume', 'toggle'):
            want = {'pause': True, 'resume': False}.get(c, not self.paused)
            if want != self.paused and self.p_fb and self.p_fb.poll() is None:
                self.p_fb.send_signal(signal.SIGUSR1)
                self.paused = want
                log('paused' if want else 'resumed')
                self.timeline('paused' if want else 'playing')
        elif c == 'seek' and len(w) > 1:
            pos = self.position()
            t = pos + float(w[1]) if w[1][0] in '+-' else float(w[1])
            t = max(0.0, min(t, self.duration - END_SLACK))
            log('seek %.1f -> %.1f' % (pos, t))
            with self.lock: self.pending = ('seek', t)
            self.interrupt(hold=True)
        elif c == 'stop':
            with self.lock: self.pending = 'stop'
            self.interrupt()
        else:
            log('unknown command %r' % line)

    def interrupt(self, hold=False):
        # ffmpeg first so its pipe is never the thing that fails, then plexfb,
        # which exits (blanking the screen, or with hold keeping its last
        # frame up for the next presenter); the main loop sees it and acts
        for p in (self.p_ff, self.p_fb):
            if p and p.poll() is None:
                try:
                    if hold and p is self.p_fb: p.send_signal(signal.SIGUSR2)
                    else: p.terminate()
                except Exception: pass

    def fifo_thread(self):
        while True:
            try:
                with open(CTL_FIFO) as f:          # blocks until a writer opens it
                    for line in f:
                        self.command(line.strip())
            except OSError as e:
                log('fifo: %r' % e); time.sleep(1)

    # ---- main loop ----
    def run(self):
        try:
            if os.path.exists(CTL_FIFO) and not st_.S_ISFIFO(os.stat(CTL_FIFO).st_mode):
                os.unlink(CTL_FIFO)
            if not os.path.exists(CTL_FIFO): os.mkfifo(CTL_FIFO)
        except OSError as e:
            log('control fifo unavailable: %r' % e)
        threading.Thread(target=self.fifo_thread, daemon=True).start()
        signal.signal(signal.SIGTERM, lambda *a: self.command('stop'))
        signal.signal(signal.SIGINT, lambda *a: self.command('stop'))

        retries = 0
        log('playing: %s, %.0f min' % (self.title, self.duration / 60))
        try: os.unlink(PLAYSTAT + '.err')
        except OSError: pass
        while True:
            self.paused = False          # a fresh presenter starts running
            try:
                self.start()
            except RuntimeError as e:
                log(str(e)); break
            except StreamError as e:
                self.kill()
                retries += 1
                if retries > RETRIES:
                    log('giving up after %d attempts to fetch the stream' % RETRIES)
                    self.show_error('Could not fetch the stream from the server.')
                    break
                log('stream not fetched (%s), retry %d/%d' % (e, retries, RETRIES))
                time.sleep(2 * retries)
                continue
            self.timeline('playing')
            tping = time.time()
            while self.p_fb.poll() is None:
                time.sleep(0.25)
                self.write_playstat()
                if time.time() - tping >= TIMELINE_S:
                    tping = time.time()
                    self.timeline('paused' if self.paused else 'playing')
            out_pos = self.last_pos = self.position()
            s = read_status()
            with self.lock:
                pend, self.pending = self.pending, None
            self.kill()
            if pend == 'stop':
                log('stopped at %.1f s' % out_pos); break
            if isinstance(pend, tuple):            # seek: new session from there
                self.offset = pend[1]; retries = 0; continue
            if s.get('eof') == 1 and out_pos >= self.duration - END_SLACK:
                log('end of stream at %.1f s' % out_pos); break
            if not self.pump and not s.get('shown') and self.p_ff.returncode not in (None, 0):
                # ffmpeg never got a frame out of the URL. Its HTTP client is not
                # the one that reached the server a moment ago; use that one.
                log('decoder could not open the stream (rc=%s); fetching it here instead' % self.p_ff.returncode)
                self.pump = True; continue
            # died early: network, server, decoder. Try again from where we were.
            retries += 1
            if retries > RETRIES:
                log('giving up after %d restarts at %.1f s' % (RETRIES, out_pos)); break
            log('pipeline ended early at %.1f s (eof=%s, ffmpeg rc=%s), restart %d/%d'
                % (out_pos, s.get('eof'), self.p_ff.returncode, retries, RETRIES))
            self.offset = max(0.0, out_pos - 1.0)
            time.sleep(2 * retries)
        self.timeline('stopped')
        try: os.unlink(PLAYSTAT)
        except OSError: pass
        self.end_session()
        # what every client does at 90%: mark watched, clear the resume point
        if self.duration and self.last_pos >= 0.9 * self.duration:
            try:
                get('/:/scrobble', {'key': self.rk, 'identifier': 'com.plexapp.plugins.library'}, timeout=10)
                log('marked watched')
            except Exception as e: log('scrobble: %r' % e)


def main():
    if len(sys.argv) < 2:
        print(__doc__); return 2
    arg = sys.argv[1]
    if arg == 'list':
        sec = int(sys.argv[2]) if len(sys.argv) > 2 else 1
        root = ET.fromstring(get('/library/sections/%d/all' % sec, {'X-Plex-Container-Size': 15}))
        for v in root.findall('Video'):
            print('rk:%s  %s (%s)  %d min' % (v.get('ratingKey'), v.get('title'), v.get('year'),
                                              int(v.get('duration', 0)) // 60000))
        return 0
    if arg == 'ctl':
        try:                                   # non-blocking: fails instead of hanging with no player
            fd = os.open(CTL_FIFO, os.O_WRONLY | os.O_NONBLOCK)
        except OSError as e:
            print('no player running (%s)' % e.strerror); return 1
        os.write(fd, (' '.join(sys.argv[2:]) + '\n').encode()); os.close(fd)
        return 0
    if arg.startswith('rk:'):
        rk = arg[3:]
    else:
        hits = find(arg)
        if not hits:
            print('no match for %r' % arg); return 1
        rk = hits[0][0]
    if len(sys.argv) > 2 and sys.argv[2].replace('.', '').isdigit():
        offset = float(sys.argv[2])
    else:
        _, dur, offset = item_info(rk)
        if offset: log('resuming from saved position %.0f s' % offset)
    os.nice(-10)
    Player(rk, offset).run()
    return 0

if __name__ == '__main__':
    sys.exit(main())
