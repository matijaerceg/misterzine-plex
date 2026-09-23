# MisterZine Plex Core - Patreon beta access

Official beta builds let anyone link Plex, browse libraries, search, change
settings and use watched-state controls. Starting or resuming a movie or episode
requires the member code for that release. Browsing theme music remains available.
Public and normal development builds play without a code. The FPGA core and
separate playback tools do not check Patreon access.

## Unlocking a beta

Find the exact version shown in the app's drawer or Options, then find its
members-only Patreon release post. The post supplies a six-digit code.

Press Play or Resume to open the early-access code screen, or select **Beta access** in
Options. Left/right selects a digit; up/down changes it. Hold up/down to repeat.
Digit changes take effect immediately, with a short rolling animation. A keyboard can type the
six digits directly, including leading zeros. Press OK to unlock or Back to cancel.
Keyboard Backspace edits and Escape cancels. The Patreon address is displayed
on the code screen and in Options. An incorrect code stays visible for
correction. Successful entry continues the selected playback automatically;
unlocking from Options simply returns to Options.

Access is saved with the installation, separately from the Plex account. Restarts,
sign-out, reinstallations that preserve settings and updates using the same access
batch keep working. A release using a new batch requires its new code. Previously
unlocked releases still work after rollback. Keep the installation's settings and
`beta-unlocks` directory when moving or backing up your installation.

The BETA badge stays visible after unlocking. Video playback has no beta watermark.

To clear saved access, select **Options → Beta access → Forget beta access** and
confirm. This removes all saved beta unlocks and legacy keys on the installation,
including access to older releases. Plex sign-in and settings are kept. Beta
playback requires unlocking again; legacy builds need their key restored.

## Creating releases

Generate a code once for a new access batch (the label below is an example):

```sh
python release/beta_release.py create-code --batch example-beta
```

The six-digit code is saved in ignored `release/private-beta/<batch>.code`.
The command prints its path, not its contents. Back up this private file securely;
never commit it or include it in the app ZIP. Read it privately when preparing the
members-only post. Generation refuses to overwrite an existing batch.

Build a beta application with an explicitly selected batch:

```sh
python release/beta_release.py build --channel beta --batch example-beta --version 0.2.0-beta.1 --id beta-example-1
python release/build_package.py --core-tree core --version 0.2.0-beta.1 --id beta-example-1
```

Use the same version and unique release ID in both commands. Match the version in
the Patreon post title. Reuse the batch for as many releases as desired and repeat
its code in each post. To require a new code, generate and select a new batch.
Version changes alone do not rotate access. The binary embeds the code's SHA-256
verifier, batch and release channel; it does not embed the code itself.

Public builds explicitly omit the access requirement:

```sh
python release/beta_release.py build --channel public --version 0.2.0 --id public-0.2.0
```

Rebuild rather than renaming a beta ZIP. Check the app's `-version` output for its
release channel and batch before distributing. Invalid beta metadata blocks
playback while still allowing browsing.

## Legacy file-key releases

Existing file-key batches remain supported. Extract their patron key ZIP onto
the SD card root, preserving `misterzine-plex/beta-keys/<batch>.key`. Return to the
app and press Play. Keep older keys for rollbacks. The release tool retains
`create-key` and can build an existing file-key batch; a batch cannot contain both
a file key and a numeric code. Numeric builds do not convert or delete old keys.

## Verification and limitations

Test incorrect codes, leading zeros, saved access, failed storage writes, Play,
Resume, episode queues, updates, rollback and public builds. Verify both CRT and
HDMI layouts on MiSTer before release. Packaging uses an explicit file list that
excludes private codes, keys and saved unlock receipts.

This is an offline convenience gate. Shared codes can be passed on, a short code
can be recovered from its verifier, and modified applications can bypass the
check. There is no membership API, expiry or device binding. Cancellation does
not disable an acquired build. Existing component licenses and modification
rights remain unchanged.
