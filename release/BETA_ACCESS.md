# MisterZine Plex Core — Patreon early access

Official beta access uses a downloadable patron key. Anyone can install the app,
link Plex and browse. Playing a movie or episode in an official beta requires
the matching key. Public builds play without a key. The FPGA core and separate
playback tools do not check Patreon access.

## Patron experience

Download the key ZIP attached to the Patreon beta post and extract it onto the
root of the MiSTer SD card. It adds `misterzine-plex/beta-keys/<batch>.key`.
Keep the folders and do not edit the key. Return to the application and press
Play; it reads the key again without requiring a restart.

Keys are shared by the patrons of a release batch. They have no expiry and no
device binding. Cancellation does not disable an acquired build. New batches
can require a new key; retain older key files for older builds and rollbacks.
There is no Patreon API connection or automatic membership check in this version.
Membership is enforced by distributing the key ZIP in a paid-members-only post.

## Creator workflow

Choose a batch label, for example `2026-09`. Generate its key once:

```sh
python release/beta_release.py create-key --batch 2026-09
```

The raw key and patron ZIP are saved in ignored `release/private-beta/`. Back up
these files privately. Never commit them or include them in a public app package.
The tool refuses to replace an existing key. Upload only the patron ZIP to the
appropriate paid Patreon tier.

Build a beta application:

```sh
python release/beta_release.py build --channel beta --batch 2026-09 --version 0.1.0-beta.1 --id beta-2026-09-1
```

Then package the app, core, notices and corresponding source:

```sh
python release/build_package.py --core-tree core --id beta-2026-09-1 --version 0.1.0-beta.1
```

Its explicit file list excludes keys. Keep package release IDs unique and use
the same version and ID in both commands. Check the application's `-version`
output on MiSTer to confirm the beta batch before distributing it.

For an unlocked public application, explicitly rebuild:

```sh
python release/beta_release.py build --channel public --version 0.1.0 --id public-0.1.0
```

Do not merely rename a beta ZIP to make a public release: the check is compiled
into the application. The hash of the random key is embedded; the key itself is
not. Normal development builds are unlocked. Partial beta build settings fail
closed at playback but still allow browsing.

## Verification

Check missing/incorrect keys, copying a valid key without restarting, Play and
Resume, autoplay, retained keys after update/rollback, and an unlocked public
build. A shared key is a convenience gate and can be copied. Existing licenses
and modification rights remain unchanged. Keep keys and campaign planning private.
