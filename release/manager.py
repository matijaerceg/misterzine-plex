#!/usr/bin/env python3
"""Install and launch a self-contained MisterZine Plex Core alpha.

Only named files are replaced. Old releases, account settings and caches are
preserved. This module also runs against a temporary card root in its tests.
"""
import argparse
import contextlib
import hashlib
import json
import mmap
import os
from pathlib import Path
import re
import shutil
import signal
import struct
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request

FF_URL = 'https://johnvansickle.com/ffmpeg/releases/ffmpeg-7.0.2-armhf-static.tar.xz'
FF_SHA = '7d41f558cb1f3395b313f8ceabed78b3731c79a0962abf405ebb5cd393e93991'
PAYLOAD = {'plexcrt', 'plexplay.py', 'plexfb', 'MisterZine Plex Core.rbf'}
LEGACY_PAYLOAD = (PAYLOAD - {'MisterZine Plex Core.rbf'}) | {'MisterZine Plex.rbf'}
SCRIPTS = {'Run': 'run', 'Rollback': 'rollback', 'Remove': 'remove', 'Diagnostics': 'diagnostics'}


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as src:
        for block in iter(lambda: src.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def atomic(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, name = tempfile.mkstemp(prefix='.' + path.name + '-', dir=str(path.parent))
    try:
        with os.fdopen(fd, 'wb') as out:
            out.write(data)
            out.flush()
            os.fsync(out.fileno())
        os.replace(name, str(path))
    finally:
        if os.path.exists(name):
            os.unlink(name)


def write_json(path, data):
    atomic(path, (json.dumps(data, indent=2) + '\n').encode())


def read_state(root):
    state = json.loads((root / 'active.json').read_text())
    for key in ('current', 'previous'):
        value = state.get(key)
        if value is not None and not re.fullmatch(r'[A-Za-z0-9_.-]+', value):
            raise ValueError('Invalid release selection; run the installer again')
    return state


@contextlib.contextmanager
def locked(root):
    import fcntl
    root.mkdir(parents=True, exist_ok=True)
    with (root / 'manager.lock').open('a') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise RuntimeError('MisterZine Plex Core is running. Return to the MiSTer menu first.')
        yield


def decoder(root, archive=None):
    ff = root / 'ffmpeg'
    stamp = root / 'decoder.json'
    if ff.is_file() and stamp.is_file():
        known = json.loads(stamp.read_text())
        if known.get('archive_sha256') == FF_SHA and digest(ff) == known.get('binary_sha256'):
            return
    if archive is None:
        archive = root / 'ffmpeg-7.0.2-armhf-static.tar.xz'
        if not archive.exists() or digest(archive) != FF_SHA:
            print('Downloading the pinned FFmpeg decoder (about 20 MB)...', flush=True)
            with urllib.request.urlopen(FF_URL, timeout=60) as response:
                atomic(archive, response.read(64 * 1024 * 1024 + 1))
    if digest(archive) != FF_SHA:
        raise ValueError('Decoder checksum failed. No release was activated.')
    with tarfile.open(str(archive), 'r:xz') as tar:
        base = 'ffmpeg-7.0.2-armhf-static/'
        for name in ('ffmpeg', 'GPLv3.txt', 'readme.txt'):
            member = tar.getmember(base + name)
            if not member.isfile() or member.size > 100 * 1024 * 1024:
                raise ValueError('Unexpected decoder archive contents')
            with tar.extractfile(member) as src:
                dest = ff if name == 'ffmpeg' else root / 'decoder-notices' / name
                atomic(dest, src.read())
    ff.chmod(0o755)
    write_json(stamp, {'archive_sha256': FF_SHA, 'binary_sha256': digest(ff), 'url': FF_URL})


def wrappers(card):
    for label, action in SCRIPTS.items():
        path = card / 'Scripts' / ('MisterZine-Plex-Core-' + label + '.sh')
        body = '#!/bin/bash\npython3 /media/fat/misterzine-plex/manager.py ' + action + '\n'
        body += 'result=$?\nif [ "$result" -ne 0 ]; then read -r -p "Press Enter to return to MiSTer..."; fi\nexit "$result"\n'
        atomic(path, body.encode())
        path.chmod(0o755)


def install(card, package, archive=None):
    root = card / 'misterzine-plex'
    manifest = json.loads((package / 'manifest.json').read_text())
    files = manifest['files']
    if set(files) != PAYLOAD:
        raise ValueError('Unexpected package file list')
    ident = manifest['id']
    if not re.fullmatch(r'[A-Za-z0-9_.-]+', ident):
        raise ValueError('Invalid release identifier')
    for name, expected in files.items():
        if digest(package / 'payload' / name) != expected:
            raise ValueError('Package checksum failed: ' + name)
    root.mkdir(parents=True, exist_ok=True)
    decoder(root, archive)
    dest = root / 'releases' / ident
    dest.mkdir(parents=True, exist_ok=True)
    for name, expected in files.items():
        target = dest / name
        if target.exists() and digest(target) != expected:
            raise ValueError('Release identifier already contains different files')
        if not target.exists():
            atomic(target, (package / 'payload' / name).read_bytes())
        target.chmod(0o755 if name != 'MisterZine Plex Core.rbf' else 0o644)
    write_json(dest / 'manifest.json', manifest)
    # Activate only after every executable and dependency has been verified.
    state = read_state(root) if (root / 'active.json').exists() else {}
    previous = state.get('previous') if state.get('current') == ident else state.get('current')
    atomic(root / 'manager.py', Path(__file__).read_bytes())
    for name in ('README.md', 'TERMS.md', 'THIRD_PARTY_NOTICES.md', 'BETA_ACCESS.md', 'corresponding-source.zip'):
        if (package / name).is_file():
            atomic(root / name, (package / name).read_bytes())
    for name in ('Apache-2.0.txt', 'Go.txt', 'GPL-2.0.txt', 'GPL-3.0.txt', 'LGPL-2.1.txt'):
        if (package / 'licenses' / name).is_file():
            atomic(root / 'licenses' / name, (package / 'licenses' / name).read_bytes())
    wrappers(card)
    write_json(root / 'active.json', {'current': ident, 'previous': previous})
    print('Installed ' + ident + '. Launch Scripts > MisterZine-Plex-Core-Run.')


def rollback(root):
    state = read_state(root)
    if not state.get('previous'):
        raise ValueError('No previous release is installed yet')
    previous = root / 'releases' / state['previous']
    manifest = json.loads((previous / 'manifest.json').read_text())
    if set(manifest['files']) not in (PAYLOAD, LEGACY_PAYLOAD):
        raise ValueError('Previous release is incomplete')
    for name, expected in manifest['files'].items():
        if digest(previous / name) != expected:
            raise ValueError('Previous release checksum failed')
    write_json(root / 'active.json', {'current': state['previous'], 'previous': state['current']})
    print('Previous release selected. Account and settings preserved.')


def remove(card):
    # Disable the launch entries; retain everything needed to recover an install.
    for label in SCRIPTS:
        path = card / 'Scripts' / ('MisterZine-Plex-Core-' + label + '.sh')
        if path.exists():
            os.replace(str(path), str(path) + '.disabled')
    print('Launch entries disabled. Settings, cache and releases remain in /media/fat/misterzine-plex.')
    print('Run the installer to restore the entries.')


def safe_log(text, secrets):
    import urllib.parse
    for secret in secrets:
        if secret:
            for value in (secret, urllib.parse.quote(secret, safe=''), urllib.parse.quote_plus(secret)):
                text = text.replace(value, '[redacted]')
    text = re.sub(r'(?i)((?:x-plex-token|authtoken|accesstoken|token)[= :"%]+)[^&\s"<>]+', r'\1[redacted]', text)
    text = re.sub(r'(?i)(authorization:\s*bearer\s+)\S+', r'\1[redacted]', text)
    text = re.sub(r'https?://[^\s"<>]+', '[server address removed]', text)
    return text


def diagnostics(root):
    secrets = []
    # Read only to redact; never copy account files into a report.
    for path in (root / 'plexcrt.json', root / 'plexcrt.json.bak'):
        try:
            cfg = json.loads(path.read_text())
            secrets += [cfg.get(k, '') for k in ('token', 'server_token', 'server_url', 'server_name', 'client_id')]
        except (OSError, ValueError):
            pass
    report = {'release': read_state(root), 'kernel': os.uname().release, 'logs': {}}
    for name in ('misterzine-plex.log', 'plexplay.log'):
        path = Path('/tmp') / name
        if path.is_file():
            with path.open('rb') as src:
                src.seek(max(0, path.stat().st_size - 64 * 1024))
                data = src.read().decode('utf-8', errors='replace')
                # Drop a potentially truncated initial credential-bearing line.
                data = data.partition('\n')[2]
            report['logs'][name] = safe_log(data, secrets)
    out = root / 'diagnostics.json'
    write_json(out, report)
    print('Report: ' + str(out))
    print('Review before sharing: media titles and playback details may appear. Account files are excluded.')


def stop_child(child):
    if child.poll() is None:
        child.terminate()
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()


def cleanup_player(folder):
    marker = ('MISTERZINE_PLEX_OWNER=' + str(folder / 'plexplay.py')).encode()
    owned = []
    for proc in Path('/proc').iterdir():
        if not proc.name.isdigit() or int(proc.name) == os.getpid():
            continue
        try:
            if marker in (proc / 'environ').read_bytes().split(b'\0'):
                os.kill(int(proc.name), signal.SIGTERM)
                owned.append(proc)
        except OSError:
            pass
    if owned:
        time.sleep(0.5)
    for proc in owned:
        try:
            if marker in (proc / 'environ').read_bytes().split(b'\0'):
                os.kill(int(proc.name), signal.SIGKILL)
        except OSError:
            pass


def prepare_framebuffer(parameters=Path('/sys/module/MiSTer_fb/parameters')):
    # MiSTer sizes fbdev for the selected HDMI/menu profile. The Plex core
    # instead uses a fixed frame ring in this reserved memory. Enlarge only
    # the Linux mapping; this does not change the core or HDMI scan timing.
    # MiSTer reapplies its own framebuffer mode when another core is loaded.
    mode = parameters / 'mode'
    values = [int(value) for value in mode.read_text().split()]
    if len(values) != 5:
        raise RuntimeError('Cannot read MiSTer framebuffer geometry')
    fmt, rb, width, height, stride = values
    if fmt != 8888 or stride * height < 0x7e0000:
        mode.write_text('8888 1 1920 1080 7680\n')


def run(root):
    state = read_state(root)
    folder = root / 'releases' / state['current']
    args = [str(folder / 'plexcrt'), '-config', str(root / 'plexcrt.json'),
            '-cache', str(root / 'cache'), '-ffmpeg', str(root / 'ffmpeg')]
    subprocess.run(args + ['-check'], check=True)
    with open('/dev/MiSTer_cmd', 'w') as cmd:
        core = folder / 'MisterZine Plex Core.rbf'
        if not core.exists():
            core = folder / 'MisterZine Plex.rbf'
        cmd.write('load_core ' + str(core) + '\n')
    time.sleep(2)
    # Keep a read-only watch on the core. Returning to Menu stops this app too.
    with open('/dev/fb0', 'rb') as fb, mmap.mmap(fb.fileno(), 4096, access=mmap.ACCESS_READ) as mem:
        last = struct.unpack_from('<I', mem, 0x40)[0]
        deadline = time.monotonic() + 8
        while time.monotonic() < deadline:
            time.sleep(0.1)
            field = struct.unpack_from('<I', mem, 0x40)[0]
            status = struct.unpack_from('<I', mem, 0x6c)[0]
            if status & 0xfffffff0 == 0x56500000 and field != last:
                break
            last = field
        else:
            raise RuntimeError('Plex core did not start. Reinstall the matching package and try again.')
        prepare_framebuffer()
        changed = time.monotonic()
        with open('/tmp/misterzine-plex.log', 'wb') as log:
            child = subprocess.Popen(args, stdout=log, stderr=log)
            def interrupted(signum, frame):
                raise KeyboardInterrupt()
            signal.signal(signal.SIGTERM, interrupted)
            try:
                while child.poll() is None:
                    time.sleep(0.5)
                    field = struct.unpack_from('<I', mem, 0x40)[0]
                    status = struct.unpack_from('<I', mem, 0x6c)[0]
                    if status & 0xfffffff0 != 0x56500000:
                        break
                    if field != last:
                        last, changed = field, time.monotonic()
                    elif time.monotonic() - changed > 2:
                        break
                if child.poll() not in (None, 0):
                    raise RuntimeError('App could not start. Run MisterZine-Plex-Core-Diagnostics and check the report.')
            finally:
                stop_child(child)
                cleanup_player(folder)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['install', 'run', 'rollback', 'remove', 'diagnostics'])
    parser.add_argument('--card', type=Path, default=Path('/media/fat'))
    parser.add_argument('--package', type=Path, default=Path(__file__).resolve().parent)
    parser.add_argument('--decoder-archive', type=Path)
    args = parser.parse_args()
    root = args.card / 'misterzine-plex'
    try:
        with locked(root):
            if args.action == 'install':
                install(args.card, args.package, args.decoder_archive)
            elif args.action == 'run':
                run(root)
            elif args.action == 'rollback':
                rollback(root)
            elif args.action == 'remove':
                remove(args.card)
            else:
                diagnostics(root)
    except KeyboardInterrupt:
        return 0
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError, tarfile.TarError) as exc:
        # Network exceptions can contain URLs. Never echo arbitrary exception text.
        if isinstance(exc, (ValueError, RuntimeError)):
            print('MisterZine Plex Core: ' + str(exc), file=sys.stderr)
        else:
            print('MisterZine Plex Core: operation failed (' + type(exc).__name__ + '). Check card space, network and package files.', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
