#!/usr/bin/env python3
"""Launch the installed app when its MiSTer main-menu entry is selected."""
import argparse
import fcntl
import hashlib
from pathlib import Path
import subprocess
import sys
import time

SELECTION = 'misterzine-plex'


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--card', type=Path, default=Path('/media/fat'))
    card = p.parse_args().card.resolve()
    root = card / 'misterzine-plex'
    key = hashlib.sha256(str(card).encode()).hexdigest()[:16]
    with open('/tmp/misterzine-plex-menu-' + key + '.lock', 'a') as singleton:
        try:
            fcntl.flock(singleton, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        handled = None
        while root.is_dir() and (root / 'menu_launcher.py').is_file():
            time.sleep(.1)
            try:
                name = Path('/tmp/CORENAME')
                stamp = name.stat().st_mtime_ns
                if stamp == handled or name.read_text().strip() != SELECTION:
                    continue
                handled = stamp
                # An activation/uninstall must finish before a menu launch.
                with (root / 'updates/worker.lock').open('a') as worker:
                    fcntl.flock(worker, fcntl.LOCK_SH)
                    with (root / 'manager.lock').open('a') as manager:
                        fcntl.flock(manager, fcntl.LOCK_EX)
                    if name.read_text().strip() != SELECTION or not (root / 'active.json').is_file():
                        continue
                    with open('/tmp/misterzine-plex-menu-run.log', 'wb') as log:
                        child = subprocess.Popen([sys.executable, str(root / 'manager.py'), 'run', '--card', str(card)],
                            stdin=subprocess.DEVNULL, stdout=log, stderr=log)
                child.wait()
            except OSError:
                time.sleep(1)


if __name__ == '__main__':
    main()
