#!/usr/bin/env python3
"""plexmenu - the PlexCRT menu: browse the server, play with plexplay.

Draws through plexui (text/rect/img into the core's frame ring) and takes
input from the core's joystick/keyboard snapshot that plexui reports, since
MiSTer main holds an exclusive grab on every evdev node.

    plexmenu.py                run
    plexmenu.py sim <word>     inject an input word into a running menu:
                               up down left right enter back info menu rew ff stop play

Pad (core build 16 button order): d-pad moves, A = select/play, B = back,
Start = play/pause, Select = stop, L/R = -60 s / +300 s, left/right while
playing = -10 s / +30 s.  Keyboard: arrows, Enter, Esc, Space, PgUp/PgDn.

Layout keeps 36 px / 24 px off the edges for CRT overscan. The CRT pixel is
8/9 as wide as it is tall, so posters are requested 135x180 to show as 2:3.
"""
import os, sys, subprocess, threading, queue, time, signal
import urllib.request, urllib.parse
import xml.etree.ElementTree as ET
sys.stdout.reconfigure(line_buffering=True)

HERE = os.path.dirname(os.path.abspath(__file__))
HOST = os.environ['PLEX_HOST']
TOKEN = os.environ['PLEX_TOKEN']
PLEXUI = os.path.join(HERE, 'plexui')
PLEXPLAY = os.path.join(HERE, 'plexplay.py')
POSTERS = '/tmp/posters'
SIM_FIFO = '/tmp/plexmenu.ctl'
PLAY_FIFO = '/tmp/plexplay.ctl'

HDRS = {'X-Plex-Token': TOKEN, 'X-Plex-Client-Identifier': 'mister-plexcrt-0001',
        'X-Plex-Product': 'Plex Web', 'X-Plex-Platform': 'Chrome',
        'X-Plex-Device-Name': 'MiSTer CRT', 'Accept': 'application/xml'}

# ---- geometry (720x480, non-square pixels) ----
SAFE_X, SAFE_Y, SAFE_W, SAFE_H = 36, 24, 648, 432
ROWS, ROW_H, ROW_Y0 = 9, 40, 88
LIST_X, LIST_W = 40, 380
SIDE_X = 440
POSTER_W, POSTER_H = 135, 180

# colours
BG, BAR, HILITE, WHITE, GREY, DIM, ACCENT = '101828', '1B2A44', '2E4A7A', 'FFFFFF', 'C8D2E0', '8A97AA', 'E5A00D'

# joystick bits (see CONF_STR J1 line): 0 right 1 left 2 down 3 up, then buttons
JOY = {0: 'right', 1: 'left', 2: 'down', 3: 'up', 4: 'enter', 5: 'back', 6: 'info', 7: 'menu',
       8: 'rew', 9: 'ff', 10: 'stop', 11: 'play'}
KEYS = {0xE075: 'up', 0xE072: 'down', 0xE06B: 'left', 0xE074: 'right', 0x5A: 'enter', 0x76: 'back',
        0x29: 'play', 0x66: 'back', 0xE07D: 'rew', 0xE07A: 'ff', 0xE071: 'stop'}


def log(*a): print(time.strftime('%H:%M:%S'), *a)

def get(path, params=None, timeout=30):
    url = HOST + path
    if params: url += ('&' if '?' in url else '?') + urllib.parse.urlencode(params)
    return urllib.request.urlopen(urllib.request.Request(url, headers=HDRS), timeout=timeout).read()


class UI:
    """plexui process: drawing commands out, input events in"""
    def __init__(self):
        self.p = subprocess.Popen([PLEXUI], stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=0)
        self.events = queue.Queue()
        self.widths = queue.Queue()
        self.joy_prev = 0
        threading.Thread(target=self._reader, daemon=True).start()

    def _reader(self):
        for raw in self.p.stdout:
            w = raw.decode(errors='replace').split()
            if not w: continue
            if w[0] == 'width': self.widths.put(int(w[1]))
            elif w[0] == 'joy':
                j = int(w[1], 16)
                pressed = j & ~self.joy_prev
                self.joy_prev = j
                for bit, name in JOY.items():
                    if pressed & (1 << bit) or pressed & (1 << (bit + 16)):
                        self.events.put((name, time.time()))
                self.events.put(('joyraw', j))          # for auto-repeat
            elif w[0] == 'key' and w[2] == '1':
                name = KEYS.get(int(w[1], 16))
                if name: self.events.put((name, time.time()))
        self.events.put(('quit', 0))

    def cmd(self, s):
        try: self.p.stdin.write((s + '\n').encode())
        except BrokenPipeError: pass

    def text(self, x, y, col, size, s): self.cmd('text %d %d %s %s %s' % (x, y, col, size, s.replace('\n', ' ')))
    def rect(self, x, y, w, h, col): self.cmd('rect %d %d %d %d %s' % (x, y, w, h, col))
    def width(self, size, s):
        self.cmd('width %s %s' % (size, s))
        try: return self.widths.get(timeout=2)
        except queue.Empty: return len(s) * (16 if size == 'big' else 11)

    def wrap(self, size, s, maxw, maxlines):
        lines, cur = [], ''
        for word in s.split():
            t = (cur + ' ' + word).strip()
            if self.width(size, t) <= maxw: cur = t
            else:
                if cur: lines.append(cur)
                cur = word
                if len(lines) == maxlines: break
        if cur and len(lines) < maxlines: lines.append(cur)
        if len(lines) == maxlines and self.width(size, ' '.join(s.split())) > maxw * maxlines:
            lines[-1] = lines[-1][:max(0, len(lines[-1]) - 3)] + '...'
        return lines

    def ellipsize(self, size, s, maxw):
        if self.width(size, s) <= maxw: return s
        while s and self.width(size, s + '...') > maxw: s = s[:-1]
        return s + '...'


class Item:
    def __init__(self, el):
        self.el = el
        self.rk = el.get('ratingKey')
        self.playable = el.tag == 'Video'
        self.key = el.get('key')                     # Directory: children path
        t = el.get('title', '?')
        if el.get('type') == 'episode':
            t = 'S%sE%s  %s' % (el.get('parentIndex', '?'), el.get('index', '?'), t)
        elif el.get('type') == 'season':
            t = el.get('title', '?')
        self.title = t
        self.year = el.get('year', '')
        self.mins = int(el.get('duration', 0)) // 60000
        self.summary = el.get('summary', '')
        self.thumb = el.get('thumb') or el.get('parentThumb') or el.get('grandparentThumb')
        self.view_offset = int(el.get('viewOffset', 0)) / 1000
        self.watched = int(el.get('viewCount', 0)) > 0
        self.leaf = el.get('leafCount'); self.viewed_leaf = el.get('viewedLeafCount')
        self.genres = ', '.join(g.get('tag') for g in el.findall('Genre')[:3])
        self.roles = ', '.join(r.get('tag') for r in el.findall('Role')[:3])
        self.grandparent = el.get('grandparentTitle', '')

    def meta_line(self):
        parts = []
        if self.year: parts.append(str(self.year))
        if self.mins: parts.append('%d min' % self.mins)
        if self.leaf: parts.append('%s/%s watched' % (self.viewed_leaf or 0, self.leaf))
        if self.genres: parts.append(self.genres)
        return '   '.join(parts)


class View:
    """a pseudo item that opens a server view"""
    def __init__(self, title, key, params=None):
        self.title, self.key, self.params = title, key, params
        self.playable = False; self.rk = None; self.thumb = None
        self.year = ''; self.mins = 0; self.summary = ''; self.view_offset = 0; self.watched = False
        self.leaf = None; self.genres = ''; self.roles = ''; self.grandparent = ''
    def meta_line(self): return ''


class Level:
    def __init__(self, title, path, params=None):
        self.title, self.path, self.params = title, path, params or {}
        self.items, self.sel, self.top = [], 0, 0

    def load(self):
        root = ET.fromstring(get(self.path, dict(self.params, **{'X-Plex-Container-Size': 2000})))
        self.items = [Item(e) for e in root if e.tag in ('Video', 'Directory')]
        return self


class Menu:
    def __init__(self):
        self.ui = UI()
        self.stack = []
        self.posters = {}                              # rk -> path or None while loading
        self.dirty = True
        self.player = None                             # subprocess while playing
        self.status = ''
        os.makedirs(POSTERS, exist_ok=True)
        threading.Thread(target=self._sim_reader, daemon=True).start()

    # ---- input from a FIFO, for tests without a pad ----
    def _sim_reader(self):
        try:
            if os.path.exists(SIM_FIFO): os.unlink(SIM_FIFO)
            os.mkfifo(SIM_FIFO)
        except OSError as e:
            log('sim fifo: %r' % e); return
        while True:
            with open(SIM_FIFO) as f:
                for line in f:
                    for w in line.split(): self.ui.events.put((w, time.time()))

    # ---- posters ----
    def poster(self, item):
        if not item or not item.thumb: return None
        p = self.posters.get(item.rk, 'new')
        if p != 'new': return p
        self.posters[item.rk] = None
        def fetch():
            path = '%s/%s.ppm' % (POSTERS, item.rk)
            try:
                if not os.path.exists(path):
                    d = get('/photo/:/transcode', {'width': POSTER_W, 'height': POSTER_H, 'minSize': 1,
                                                   'upscale': 1, 'format': 'ppm', 'url': item.thumb}, timeout=20)
                    if not d.startswith(b'P6'): raise ValueError('not ppm')
                    open(path + '.part', 'wb').write(d); os.rename(path + '.part', path)
                self.posters[item.rk] = path
            except Exception as e:
                log('poster %s: %r' % (item.rk, e)); self.posters[item.rk] = ''
            self.dirty = True
        threading.Thread(target=fetch, daemon=True).start()
        return None

    # ---- drawing ----
    def draw(self):
        ui, lv = self.ui, self.stack[-1]
        ui.cmd('fill ' + BG)
        ui.rect(0, SAFE_Y, 720, 48, BAR)
        crumb = ' / '.join(l.title for l in self.stack[-2:])
        ui.text(LIST_X, SAFE_Y + 8, WHITE, 'big', ui.ellipsize('big', crumb, 480))
        if lv.items:
            ui.text(560, SAFE_Y + 14, GREY, 'small', '%d / %d' % (lv.sel + 1, len(lv.items)))
        if not lv.items:
            ui.text(LIST_X, ROW_Y0, GREY, 'big', 'Nothing here')
        for i in range(ROWS):
            n = lv.top + i
            if n >= len(lv.items): break
            it, y = lv.items[n], ROW_Y0 + i * ROW_H
            if n == lv.sel: ui.rect(LIST_X - 8, y - 2, LIST_W + 16, ROW_H - 4, HILITE)
            col = WHITE if n == lv.sel else (DIM if it.watched else GREY)
            ui.text(LIST_X, y, col, 'big', ui.ellipsize('big', it.title, LIST_W - 30))
            if it.playable and it.view_offset and not it.watched:
                ui.rect(LIST_X + LIST_W - 12, y + 10, 8, 12, ACCENT)      # resume marker
            elif it.playable and it.watched:
                ui.rect(LIST_X + LIST_W - 12, y + 12, 8, 8, DIM)
        if lv.items:
            it = lv.items[lv.sel]
            p = self.poster(it)
            ui.rect(SIDE_X, ROW_Y0, POSTER_W, POSTER_H, '000000')
            if p: ui.cmd('img %d %d %s' % (SIDE_X, ROW_Y0, p))
            y = ROW_Y0 + POSTER_H + 12
            for line in ui.wrap('small', it.grandparent or it.title, SAFE_X + SAFE_W - SIDE_X, 2):
                ui.text(SIDE_X, y, WHITE, 'small', line); y += 24
            ui.text(SIDE_X, y, GREY, 'small', ui.ellipsize('small', it.meta_line(), SAFE_X + SAFE_W - SIDE_X)); y += 28
            for line in ui.wrap('small', it.summary, SAFE_X + SAFE_W - SIDE_X, 4):
                ui.text(SIDE_X, y, DIM, 'small', line); y += 24
            if it.view_offset and not it.watched:
                ui.text(SIDE_X, SAFE_Y + SAFE_H - 24, ACCENT, 'small', 'Resume at %d:%02d' % divmod(int(it.view_offset), 60))
        if self.status:
            ui.text(LIST_X, SAFE_Y + SAFE_H - 24, ACCENT, 'small', self.status)
        ui.cmd('show')
        self.dirty = False

    def draw_message(self, msg):
        ui = self.ui
        ui.cmd('fill ' + BG)
        ui.text(LIST_X, 220, WHITE, 'big', msg)
        ui.cmd('show')

    # ---- navigation ----
    def enter(self, item):
        if item.playable:
            self.play(item); return
        if not item.key.startswith('/'):
            # a library section: offer the views a big library needs before an
            # alphabetical list of thousands
            sec = item.key
            lv = Level(item.title, None)
            lv.items = [View('Continue Watching', '/library/sections/%s/onDeck' % sec),
                        View('Recently Added', '/library/sections/%s/recentlyAdded' % sec),
                        View('Recently Released', '/library/sections/%s/all' % sec, {'sort': 'originallyAvailableAt:desc'}),
                        View('Unwatched', '/library/sections/%s/all' % sec, {'unwatched': 1}),
                        View('All, A to Z', '/library/sections/%s/all' % sec)]
            self.stack.append(lv); self.dirty = True; return
        self.draw_message('Loading %s ...' % item.title)
        try:
            self.stack.append(Level(item.title, item.key, getattr(item, 'params', None)).load())
        except Exception as e:
            log('load: %r' % e); self.status = 'Could not load: %s' % e
        self.dirty = True

    def move(self, d):
        lv = self.stack[-1]
        if not lv.items: return
        lv.sel = max(0, min(len(lv.items) - 1, lv.sel + d))
        if lv.sel < lv.top: lv.top = lv.sel
        if lv.sel >= lv.top + ROWS: lv.top = lv.sel - ROWS + 1
        self.dirty = True

    # ---- playback ----
    def play(self, item):
        self.draw_message('Starting %s ...' % item.title)
        logf = open('/tmp/plexplay.log', 'a')
        self.player = subprocess.Popen([sys.executable, PLEXPLAY, 'rk:' + item.rk], stdout=logf, stderr=logf)
        log('playing rk %s' % item.rk)
        t0 = time.time()
        while self.player.poll() is None:
            try: ev, arg = self.ui.events.get(timeout=0.25)
            except queue.Empty: continue
            c = {'play': 'toggle', 'enter': 'toggle', 'back': 'stop', 'stop': 'stop',
                 'left': 'seek -10', 'right': 'seek +30', 'rew': 'seek -60', 'ff': 'seek +300'}.get(ev)
            if c and time.time() - t0 > 1.0: self.player_cmd(c)
        self.player.wait(); self.player = None
        # refresh watched state / resume point of the item we just played
        try:
            el = ET.fromstring(get('/library/metadata/%s' % item.rk)).find('Video')
            if el is not None:
                new = Item(el); item.view_offset, item.watched = new.view_offset, new.watched
        except Exception as e: log('refresh: %r' % e)
        while not self.ui.events.empty(): self.ui.events.get_nowait()     # drop stale presses
        self.ui.joy_prev = 0
        self.dirty = True

    def player_cmd(self, c):
        try:
            fd = os.open(PLAY_FIFO, os.O_WRONLY | os.O_NONBLOCK)
            os.write(fd, (c + '\n').encode()); os.close(fd)
        except OSError as e:
            log('player cmd %s: %r' % (c, e))

    # ---- main loop ----
    def run(self):
        self.draw_message('Connecting to Plex ...')
        try:
            root = Level('Plex', '/library/sections').load()
            root.items = [i for i in root.items if i.el.get('type') in ('movie', 'show')]
        except Exception as e:
            self.draw_message('Cannot reach the server: %s' % e); time.sleep(5); return
        self.stack.append(root)
        held, held_since, last_rep = None, 0, 0
        while True:
            if self.dirty: self.draw()
            try: ev, arg = self.ui.events.get(timeout=0.05)
            except queue.Empty: ev, arg = None, None
            now = time.time()
            if ev == 'joyraw':
                d = None
                for bit, name in ((3, 'up'), (2, 'down'), (1, 'left'), (0, 'right')):
                    if arg & (1 << bit) or arg & (1 << (bit + 16)): d = name
                if d != held: held, held_since, last_rep = d, now, now
                continue
            if held and now - held_since > 0.35 and now - last_rep > 0.08:      # auto-repeat
                last_rep = now; ev = held
            if not ev: continue
            self.status = ''
            lv = self.stack[-1]
            if ev == 'quit': break
            elif ev == 'up': self.move(-1)
            elif ev == 'down': self.move(1)
            elif ev == 'left': self.move(-ROWS)
            elif ev == 'right': self.move(ROWS)
            elif ev == 'rew': self.move(-10 * ROWS)
            elif ev == 'ff': self.move(10 * ROWS)
            elif ev in ('enter', 'play'):
                if lv.items: self.enter(lv.items[lv.sel])
            elif ev == 'back':
                if len(self.stack) > 1: self.stack.pop(); self.dirty = True
            elif ev == 'menu':
                while len(self.stack) > 1: self.stack.pop()
                self.dirty = True
        self.ui.cmd('quit')


def main():
    if len(sys.argv) > 2 and sys.argv[1] == 'sim':
        fd = os.open(SIM_FIFO, os.O_WRONLY | os.O_NONBLOCK)
        os.write(fd, (' '.join(sys.argv[2:]) + '\n').encode()); os.close(fd)
        return 0
    os.nice(-5)
    m = Menu()
    signal.signal(signal.SIGTERM, lambda *a: sys.exit(0))
    try: m.run()
    finally:
        if m.player and m.player.poll() is None: m.player_cmd('stop'); m.player.wait()
        m.ui.cmd('blank'); m.ui.cmd('quit')
    return 0

if __name__ == '__main__':
    sys.exit(main())
