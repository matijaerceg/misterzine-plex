#!/usr/bin/env python3
"""Create patron key ZIPs and explicitly build public or beta applications."""
import argparse
import hashlib
import os
from pathlib import Path
import re
import secrets
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent.parent
PRIVATE = ROOT / 'release/private-beta'


def batch_name(value):
    if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,47}', value):
        raise ValueError('Batch must be 1-48 lowercase letters, digits or hyphens, starting with a letter or digit')
    return value


def create_key(batch, private=PRIVATE):
    batch_name(batch)
    private.mkdir(parents=True, exist_ok=True)
    path = private / (batch + '.key')
    archive = private / (batch + '-patron-key.zip')
    if archive.exists():
        raise FileExistsError('A patron ZIP already exists for this batch; restore its original key or choose a new batch')
    # Never replace a key: existing releases depend on its exact bytes.
    with path.open('xb') as out:
        out.write(secrets.token_bytes(32))
    with zipfile.ZipFile(archive, 'x', zipfile.ZIP_DEFLATED) as out:
        out.write(path, 'misterzine-plex/beta-keys/' + path.name)
        out.writestr('BETA-KEY-README.txt',
                     'MisterZine Plex Core - Patreon beta access\n\n'
                     'Extract this ZIP onto the root of your MiSTer SD card.\n'
                     'Keep the misterzine-plex folder structure. Do not edit the .key file.\n'
                     'Return to the app and press Play again. No restart is needed.\n'
                     'Keep older keys to keep older beta releases working.\n'
                     'This key unlocks beta batch ' + batch + '; it does not expire.\n')
    return archive


def build(channel, batch, go, version, ident, private=PRIVATE):
    flags = ['-s', '-w']
    # Metadata is passed inside Go's linker argument syntax. Reject whitespace
    # and quotes so it cannot introduce additional linker options.
    for value in (version, ident):
        if not re.fullmatch(r'[A-Za-z0-9_.-]+', value):
            raise ValueError('Version and build ID must use letters, digits, dots, underscores or hyphens')
    flags += ['-X', 'main.version=' + version, '-X', 'main.build=' + ident]
    if channel == 'beta':
        batch_name(batch)
        key = (private / (batch + '.key')).read_bytes()
        if len(key) != 32:
            raise ValueError('Beta key must be exactly 32 bytes')
        flags += ['-X', 'plexcrt/internal/beta.Batch=' + batch,
                  '-X', 'plexcrt/internal/beta.KeySHA256=' + hashlib.sha256(key).hexdigest()]
    elif batch:
        raise ValueError('Public builds must not specify a beta batch')
    env = dict(os.environ, GOOS='linux', GOARCH='arm', GOARM='7', CGO_ENABLED='0')
    subprocess.run([go, 'build', '-trimpath', '-ldflags', ' '.join(flags),
                    '-o', 'dist/plexcrt', './cmd/plexcrt'], cwd=ROOT / 'app', env=env, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    key = commands.add_parser('create-key')
    key.add_argument('--batch', required=True)
    app = commands.add_parser('build')
    app.add_argument('--channel', choices=['public', 'beta'], required=True)
    app.add_argument('--batch', default='')
    app.add_argument('--go', default='go')
    app.add_argument('--version', required=True)
    app.add_argument('--id', required=True)
    args = parser.parse_args()
    if args.command == 'create-key':
        print('Patron-only ZIP:', create_key(args.batch))
    else:
        build(args.channel, args.batch, args.go, args.version, args.id)
        print('Built app/dist/plexcrt:', args.channel, args.batch)


if __name__ == '__main__':
    main()
