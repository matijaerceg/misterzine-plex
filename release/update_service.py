#!/usr/bin/env python3
"""Independent Plex installation/update worker. Downloader owns staging only.

Downloader discovery follows the approach documented in MisterZine on-device:
https://github.com/matijaerceg/misterzine-on-device/tree/main/internal/updater
No Update All execution is needed. Downloader itself retains its GPL-3.0 license.
"""
import argparse
import contextlib
import hashlib
import io
import json
import lzma
import os
import platform
from pathlib import Path, PurePosixPath
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time
import urllib.request
import zipfile

import catalogue as releases
import manager

BOOTSTRAP_URL = 'https://raw.githubusercontent.com/MiSTer-devel/Downloader_MiSTer/5d0771359ae396aaea64453e6791ac87781d78f4/dont_download.sh'
BOOTSTRAP_SHA = 'e19ed080deb4ac67f646e4318438427243e2f65fb95c0a1b6e738be4d8ad4903'
BOOTSTRAP_ZIP_SHA = '93c247f04c0082ba724abbd6e041a166351c7c758fe114e52447575aae9a65ed'
HELPERS = manager.HELPERS
SCRIPT_NAMES = ('Run', 'Rollback', 'Diagnostics', 'Uninstall')


def root_for(card):
    card = Path(card).resolve()
    if not card.is_dir() or card == Path('/'):
        raise ValueError('Choose a mounted MiSTer card directory')
    root = card / 'misterzine-plex'
    for path in (root, card / releases.STAGING, card / 'Scripts', root / 'releases', root / 'updates'):
        if path.is_symlink():
            raise ValueError('Plex installation paths cannot be symbolic links')
    return root


def guarded(root, relative):
    path = root / relative
    if path.is_symlink() or not path.resolve().is_relative_to(root.resolve()):
        raise ValueError('Unsafe installation path')
    return path


@contextlib.contextmanager
def worker_lock(root):
    import fcntl
    folder = guarded(root, 'updates')
    folder.mkdir(parents=True, exist_ok=True)
    with (folder / 'worker.lock').open('a') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise RuntimeError('A Plex update is already running')
        yield


def status(root, stage, release=None, message=''):
    if not message:
        message = {'download': 'Downloading release...', 'verify': 'Verifying download...',
                   'install': 'Installing release files...'}.get(stage, '')
    value = {'stage': stage, 'message': message, 'pid': os.getpid(), 'updated': time.time()}
    if release is not None:
        value['release'] = release
    manager.write_json(root / 'updates/status.json', value)
    print(stage.capitalize() + (': ' + message if message else ''), flush=True)


def other_downloader(proc_root=Path('/proc')):
    names = {'update_all.sh', 'update_all.pyz', 'ua_downloader_bin', 'ua_downloader_dd.pyz',
             'ua_downloader_latest.zip', 'downloader_bin', 'downloader.sh', 'update.sh', 'downloader_latest.zip'}
    for proc in proc_root.iterdir():
        if not proc.name.isdigit() or int(proc.name) == os.getpid():
            continue
        try:
            args = (proc / 'cmdline').read_bytes().decode(errors='replace').split('\0')
        except OSError:
            continue
        if any(Path(a).name in names for a in args):
            return True
    return False


def engine(card, bootstrap=True):
    script = card / 'Scripts/downloader.sh'
    base = card / 'Scripts/.config/downloader'
    if script.is_file():
        return ['/bin/bash', str(script)]
    binary = base / 'downloader_bin'
    if binary.is_file() and os.access(binary, os.X_OK):
        return [str(binary)]
    archive = base / 'downloader_latest.zip'
    if archive.is_file():
        return [sys.executable, str(archive)]
    if not bootstrap:
        raise RuntimeError('Downloader is not installed')
    print('Installing MiSTer Downloader...', flush=True)
    with urllib.request.urlopen(BOOTSTRAP_URL, timeout=30) as response:
        releases.https(response.geturl())
        raw = response.read(2 * 1024 * 1024)
    if hashlib.sha256(raw).hexdigest() != BOOTSTRAP_SHA:
        raise ValueError('Downloader bootstrap verification failed')
    data = lzma.decompress(raw.split(b'\n', 7)[7])
    if hashlib.sha256(data).hexdigest() != BOOTSTRAP_ZIP_SHA or not zipfile.is_zipfile(io.BytesIO(data)):
        raise ValueError('Downloader archive verification failed')
    manager.atomic(archive, data)
    return [sys.executable, str(archive)]


def database_text(release):
    return '[misterzine_plex]\ndb_url = ' + release['db_url'] + '\nfilter =\n'


def register(card, release):
    # Only this owned drop-in is modified. Existing global configuration survives.
    manager.atomic(card / 'downloader_misterzine_plex.ini', database_text(release).encode())


def download(card, root, release, runner=subprocess.run):
    if other_downloader():
        raise RuntimeError('Another Downloader or Update All run is active. Try again when it finishes.')
    command = engine(card)
    ini = root / 'updates/downloader.ini'
    manager.atomic(ini, ('[MiSTer]\nupdate_linux=false\nallow_reboot=0\nstorage_priority=off\n' + database_text(release)).encode())
    env = dict(os.environ, DOWNLOADER_INI_PATH=str(ini), DOWNLOADER_LAUNCHER_PATH=str(card / 'Scripts/downloader.sh'),
               FORCED_BASE_PATH=str(card), DEFAULT_BASE_PATH=str(card), UPDATE_LINUX='false', ALLOW_REBOOT='0',
               EXTRA_DROP_IN_DATABASE_FILES='', FAIL_ON_FILE_ERROR='true', PYTHONUTF8='1',
               LOGFILE=str(root / 'updates/downloader.log'))
    for cert in (card / 'Scripts/.config/downloader/cacert.pem', Path('/etc/ssl/certs/cacert.pem')):
        if cert.is_file():
            env['SSL_CERT_FILE'] = str(cert)
            env['CURL_SSL'] = '--cacert ' + str(cert)
            break
    # Output belongs in an explicit device diagnostic log, never in the UI or request file.
    with (root / 'updates/download-output.log').open('wb') as log:
        result = runner(command + ['--run-only', releases.DB_ID], env=env, stdout=log, stderr=log, timeout=1800)
    if result.returncode:
        raise RuntimeError('Download failed. Check network and free space, then retry.')


def unpack(archive, destination, release):
    if archive.stat().st_size != release['size'] or manager.digest(archive) != release['sha256']:
        raise ValueError('Downloaded release changed or failed verification. Check for updates again.')
    with zipfile.ZipFile(archive) as z:
        seen, total = set(), 0
        for info in z.infolist():
            path = PurePosixPath(info.filename)
            if (path.is_absolute() or '..' in path.parts or '\\' in info.filename or ':' in info.filename
                    or info.filename in seen or stat.S_ISLNK(info.external_attr >> 16)):
                raise ValueError('Unsafe release archive')
            seen.add(info.filename)
            total += info.file_size
            if total > 512 * 1024 * 1024 or len(seen) > 4096:
                raise ValueError('Release archive is too large')
        z.extractall(destination)
    package = destination / 'misterzine-plex-alpha'
    manifest = json.loads((package / 'manifest.json').read_text())
    if not releases.manifest_matches(manifest, release):
        raise ValueError('Package does not match the selected release. Check for updates again.')
    if set(manifest.get('files', {})) != manager.PAYLOAD:
        raise ValueError('Unexpected package payload')
    for name, sha in manifest['files'].items():
        if not releases.HASH.fullmatch(str(sha)) or manager.digest(package / 'payload' / name) != sha:
            raise ValueError('Package payload failed verification')
    for name in HELPERS:
        if not (package / name).is_file():
            raise ValueError('Package is missing installation support')
    return package


def prepare(card, release, downloader=download):
    root = root_for(card)
    releases.entry(release)
    with worker_lock(root):
        try:
            status(root, 'download', release)
            archive = guarded(card, releases.STAGING + '/package.zip')
            # A matching package from an ordinary Downloader run can be reused.
            if not archive.is_file() or archive.stat().st_size != release['size'] or manager.digest(archive) != release['sha256']:
                downloader(card, root, release)
            status(root, 'verify', release)
            with tempfile.TemporaryDirectory(prefix='package-', dir=str(root / 'updates')) as tmp:
                # External Downloader runs may replace staging at any time. Verify
                # and extract one private snapshot, never reopen the shared file.
                snapshot = Path(tmp) / 'selected.zip'
                shutil.copyfile(archive, snapshot)
                package = unpack(snapshot, Path(tmp), release)
                status(root, 'install', release)
                manager.stage(card, package)
                # Ready package survives the worker and contains verified maintenance files.
                ready = guarded(root, 'updates/ready')
                if ready.exists():
                    shutil.rmtree(ready)
                shutil.copytree(package, ready)
                manager.write_json(root / 'updates/ready-hashes.json', {
                    name: manager.digest(ready / name) for name in HELPERS + ('manifest.json',)})
            manager.write_json(root / 'updates/ready.json', release)
            status(root, 'ready', release, 'Ready to restart')
        except Exception:
            status(root, 'failed', release, 'Update could not be prepared. Your current version will keep working.')
            raise


def manager_processes(root, proc_root=Path('/proc')):
    found = []
    for proc in proc_root.iterdir():
        if not proc.name.isdigit() or int(proc.name) == os.getpid():
            continue
        try:
            args = (proc / 'cmdline').read_bytes().split(b'\0')
        except OSError:
            continue
        if len(args) >= 3 and args[1:3] == [str(root / 'manager.py').encode(), b'run']:
            found.append(int(proc.name))
    return found


def stop_manager(root):
    pids = manager_processes(root)
    for pid in pids:
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
    deadline = time.monotonic() + 12
    def running(pid):
        try:
            # An exited child can remain in /proc until its parent reaps it.
            # It no longer owns the manager lock or app process at this point.
            state = Path('/proc', str(pid), 'stat').read_text().rsplit(')', 1)[1].split()[0]
            if state == 'Z':
                try:
                    os.waitpid(pid, os.WNOHANG)
                except ChildProcessError:
                    pass
                return False
            return True
        except FileNotFoundError:
            return False
    while any(running(pid) for pid in pids):
        if time.monotonic() > deadline:
            raise RuntimeError('Plex has not stopped yet. Return to the MiSTer menu and retry.')
        time.sleep(.1)


def start_and_check(root, timeout=25):
    ready = root / 'updates/started'
    ready.unlink(missing_ok=True)
    env = dict(os.environ, MISTERZINE_PLEX_READY_FILE=str(ready))
    child = subprocess.Popen([sys.executable, str(root / 'manager.py'), 'run', '--card', str(root.parent)],
                             stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                             start_new_session=True, env=env)
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if child.poll() is not None:
            return False
        if ready.is_file():
            time.sleep(2)
            return child.poll() is None
        time.sleep(.2)
    stop_manager(root)
    return False


def activate(card, launch=start_and_check):
    root = root_for(card)
    with worker_lock(root):
        release = releases.entry(json.loads((root / 'updates/ready.json').read_text()))
        package = root / 'updates/ready'
        verified = json.loads((root / 'updates/ready-hashes.json').read_text())
        if set(verified) != set(HELPERS + ('manifest.json',)) or any(
                manager.digest(package / name) != digest for name, digest in verified.items()):
            raise ValueError('Prepared installation support changed. Check for updates again.')
        manifest = json.loads((package / 'manifest.json').read_text())
        if not releases.manifest_matches(manifest, release):
            raise ValueError('Prepared release metadata changed')
        old = manager.read_state(root) if (root / 'active.json').exists() else None
        # Copy runtime before activation so recovery does not depend on the new helper.
        backups = {name: (root / name).read_bytes() for name in HELPERS if (root / name).exists()}
        dropin = card / 'downloader_misterzine_plex.ini'
        old_registration = dropin.read_bytes() if dropin.exists() else None
        for name, sha in manifest.get('files', {}).items():
            if name not in manager.PAYLOAD or manager.digest(package / 'payload' / name) != sha:
                raise ValueError('Prepared payload changed. Check for updates again.')
        stopped = False
        status(root, 'activating', release, 'Restarting Plex...')
        try:
            recovery = root / 'updates/recovery'
            recovery.mkdir(exist_ok=True)
            for name, data in backups.items():
                manager.atomic(recovery / name, data)
            manager.write_json(recovery / 'selection.json', old)
            manager.atomic(recovery / 'registration', old_registration or b'')
            manager.write_json(root / 'updates/activation.json', {'helpers': list(backups)})
            stop_manager(root)
            stopped = True
            with manager.locked(root):
                manager.install(card, package)
                register(card, release)
            if not launch(root):
                raise RuntimeError('The new release did not start')
        except Exception:
            if not stopped:
                (root / 'updates/activation.json').unlink(missing_ok=True)
                status(root, 'failed', release, 'Could not restart Plex. Your current version will keep working.')
                raise
            stop_manager(root)
            with manager.locked(root):
                for name, data in backups.items():
                    manager.atomic(root / name, data)
                if old is not None:
                    manager.write_json(root / 'active.json', old)
                else:
                    (root / 'active.json').unlink(missing_ok=True)
                if old_registration is None:
                    dropin.unlink(missing_ok=True)
                else:
                    manager.atomic(dropin, old_registration)
                (root / 'updates/activation.json').unlink(missing_ok=True)
            status(root, 'failed', release, 'Startup failed. The previous release was restored.' if old else 'Startup failed. Run Install to retry.')
            if old:
                if launch is start_and_check:
                    # Older releases do not emit a readiness marker. Restore them
                    # without terminating a healthy app for lacking that marker.
                    subprocess.Popen([sys.executable, str(root / 'manager.py'), 'run', '--card', str(card)],
                                     stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                     stderr=subprocess.DEVNULL, start_new_session=True)
                else:
                    launch(root)
            raise
        (root / 'updates/ready.json').unlink(missing_ok=True)
        (root / 'updates/activation.json').unlink(missing_ok=True)
        status(root, 'complete', release, 'Update installed')


def uninstall(card, keep=True):
    root = root_for(card)
    if other_downloader():
        raise RuntimeError('Wait for Downloader or Update All to finish before uninstalling.')
    with worker_lock(root):
        stop_manager(root)
        manager.stop_menu_launcher(card)
        with manager.locked(root):
            manager.menu_entries(card, enable=False)
            (card / 'downloader_misterzine_plex.ini').unlink(missing_ok=True)
            # These are exact, application-owned names, never a wildcard over Scripts.
            for prefix in ('MisterZine-Plex-', 'MisterZine-Plex-Core-'):
                for label in ('Run', 'Install', 'Install-Public', 'Install-Beta', 'Uninstall', 'Rollback', 'Diagnostics', 'Remove'):
                    for suffix in ('.sh', '.sh.disabled'):
                        path = card / 'Scripts' / (prefix + label + suffix)
                        path.unlink(missing_ok=True)
            (card / '.misterzine-plex-core.mgl').unlink(missing_ok=True)
            staged = guarded(card, releases.STAGING)
            if staged.exists():
                shutil.rmtree(staged)
            (root / 'active.json').unlink(missing_ok=True)
            if keep:
                for name in ('releases', 'updates', 'ffmpeg', 'decoder.json', 'decoder-notices', 'ffmpeg-7.0.2-armhf-static.tar.xz'):
                    path = guarded(root, name)
                    if path.is_dir():
                        shutil.rmtree(path)
                    else:
                        path.unlink(missing_ok=True)
                for name in HELPERS:
                    (root / name).unlink(missing_ok=True)
            else:
                shutil.rmtree(root)
    print('Plex removed. Settings and acquired codes kept.' if keep else 'Plex and its saved data removed.')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=('install', 'prepare', 'activate', 'uninstall'))
    p.add_argument('--card', type=Path, default=Path('/media/fat'))
    p.add_argument('--request', type=Path)
    p.add_argument('--catalogue', default=releases.CATALOGUE_URL)
    p.add_argument('--channel', choices=('public', 'beta'))
    p.add_argument('--yes', action='store_true', help='Install the selected release without prompting')
    p.add_argument('--download-db-url', help='Immutable database for a pinned installer')
    args = p.parse_args()
    if args.action == 'install' and (platform.system() != 'Linux' or platform.machine() != 'armv7l' or not Path('/dev/MiSTer_cmd').exists()):
        raise RuntimeError('Installation requires MiSTer Linux on supported ARM hardware.')
    root = root_for(args.card)
    if args.action == 'uninstall':
        print('1. Remove Plex, keep settings and codes (default)\n2. Remove Plex and all its saved data\n3. Cancel')
        choice = input('Choice [1]: ').strip()
        if choice in ('', '1'):
            uninstall(args.card)
        elif choice == '2' and input('Type REMOVE to delete all Plex data: ').strip() == 'REMOVE':
            uninstall(args.card, keep=False)
        return
    if args.action == 'activate':
        activate(args.card)
        return
    if args.request:
        release = releases.entry(json.loads(args.request.read_text()))
    else:
        available = releases.fetch(args.catalogue)['releases']
        channel = args.channel or ('public' if 'public' in available else 'beta')
        if channel not in available:
            raise ValueError('No release is available for that channel')
        release = available[channel]
        print('MisterZine Plex - ' + release['version'])
        if channel == 'beta':
            print('Playback in this early-access release requires a paid Patreon code.')
        if not args.yes and input('Install this release? [Y/n] ').strip().lower() not in ('', 'y', 'yes'):
            return
    if args.download_db_url:
        releases.https(args.download_db_url)
        def pinned_download(card, root, selected):
            download(card, root, dict(selected, db_url=args.download_db_url))
        prepare(args.card, release, pinned_download)
    else:
        prepare(args.card, release)
    if args.action == 'install':
        activate(args.card)


def cli():
    try:
        main()
    except (OSError, ValueError, RuntimeError, KeyError, subprocess.SubprocessError, zipfile.BadZipFile) as exc:
        # Do not echo URLs, credentials, or arbitrary child output.
        print(str(exc) if isinstance(exc, (ValueError, RuntimeError)) else 'Operation failed. Check network, free space and the installed package.', file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    cli()
