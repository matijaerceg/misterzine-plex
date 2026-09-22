#!/usr/bin/env python3
"""Main-menu helper for the installed MisterZine Plex Core development build."""
import fcntl
import subprocess
import time
from pathlib import Path

ROOT = Path('/media/fat/misterzine-plex')
SELECTION = 'misterzine-plex'


def main():
    # Boot hooks and manual starts can overlap; only one helper may run.
    with open('/tmp/misterzine-plex-menu.lock', 'a') as singleton:
        try:
            fcntl.flock(singleton, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        handled = None
        while True:
            time.sleep(0.1)
            try:
                name = Path('/tmp/CORENAME')
                stamp = name.stat().st_mtime_ns
                if stamp == handled or name.read_text().strip() != SELECTION:
                    continue
                handled = stamp
                # Loading the menu MGL stops any previous app session. Wait
                # for its manager to finish cleanup before launching again.
                with (ROOT / 'manager.lock').open('a') as manager_lock:
                    fcntl.flock(manager_lock, fcntl.LOCK_EX)
                if name.read_text().strip() != SELECTION:
                    continue
                with open('/tmp/misterzine-plex-menu-run.log', 'wb') as log:
                    subprocess.run(['python3', str(ROOT / 'manager.py'), 'run'],
                                   stdin=subprocess.DEVNULL, stdout=log, stderr=log)
            except OSError as error:
                print(type(error).__name__, flush=True)
                time.sleep(1)


if __name__ == '__main__':
    main()
