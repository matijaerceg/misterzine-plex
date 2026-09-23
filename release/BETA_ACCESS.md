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

## Older file-key builds

If your older release post supplies a patron key ZIP, extract it onto the SD
card root, preserving `misterzine-plex/beta-keys/<batch>.key`, then press Play.
Keep older keys for rollback. Numeric-code builds do not convert or delete them.

## Does access expire?

There is no automatic membership check, expiry or device binding. Cancellation
does not disable an acquired build. A later release may require a new code.
Existing component licenses and modification rights remain unchanged.
