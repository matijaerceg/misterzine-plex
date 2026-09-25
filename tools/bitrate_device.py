"""Run generated samples through the production decoder/presenter on MiSTer."""
import json
import os
from pathlib import Path
import subprocess
import signal
import sys
import time

root = Path(os.environ.get('BENCH_ROOT', '/media/fat/misterzine-plex-bench'))
ff = os.environ.get('FFMPEG', '/media/fat/misterzine-plex/ffmpeg')
fb = os.environ['PLEXFB']
results = []
os.setpriority(os.PRIO_PROCESS, 0, -15)  # UI thread -5, then launcher nice(-10)
signal.signal(signal.SIGTERM, lambda signum, frame: (_ for _ in ()).throw(KeyboardInterrupt()))
lease = subprocess.Popen([os.environ['BITRATEBENCH']])
try:
    time.sleep(0.5)
    if lease.poll() is not None:
        raise RuntimeError('Core lease could not start')
    for sample in json.loads((root / 'samples.json').read_text()):
        if len(sys.argv) > 1 and sample['requested_kbps'] not in [int(x) for x in sys.argv[1:]]:
            continue
        name = sample['file']
        env = dict(os.environ, PLEXFB_DEV='/dev/fb0', PLEXFB_STATUS=str(root/'status.txt'),
                   MISTERZINE_PLEX_OWNER=str(root/'bitrate_device.py'))
        with (root / (name+'.log')).open('w') as log:
            decode = subprocess.Popen([ff, '-nostdin', '-loglevel', 'warning', '-threads', '2', '-i', str(root/name),
                       '-map', '0:v:0', '-c:v', 'rawvideo', '-pix_fmt', 'yuv420p', '-map', '0:a:0', '-ac', '2', '-ar', '48000',
                       '-c:a', 'pcm_s16le', '-f', 'avi', 'pipe:1'], stdout=subprocess.PIPE, stderr=log, env=env)
            present = subprocess.Popen([fb, 'avi', str(sample['fps'])], stdin=decode.stdout, stdout=log, stderr=log, env=env)
            decode.stdout.close()
            try:
                present.wait(timeout=90)
                decode.wait(timeout=5)
            finally:
                for child in (decode, present):
                    if child.poll() is None:
                        child.kill()
                        child.wait()
        result = dict(sample, decoder_exit=decode.returncode, presenter_exit=present.returncode,
                      status=(root/'status.txt').read_text().strip(), log=(root/(name+'.log')).read_text())
        results.append(result)
        summary = {k: v for k, v in result.items() if k != 'log'}
        summary['presenter_summary'] = result['log'].splitlines()[-1]
        summary['audio_underruns'] = result['log'].count('underrun!!!')
        print(json.dumps(summary), flush=True)
        time.sleep(1)
finally:
    lease.terminate()
    lease.wait(timeout=3)
    (root / ('results-' + ('-'.join(sys.argv[1:]) or 'all') + '.json')).write_text(json.dumps(results, indent=2))
