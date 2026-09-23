# Reference

Detailed behavior and display settings. For everyday help, see the
[help page](../release/README.md).

## Using Plex


### Controls

- D-pad moves the selection; OK opens it; Back goes back or opens the drawer.
- In the library wall, L/R jumps by sort letter or page. Back selects the view
  tabs, then returns to the previous screen.
- During playback, OK opens the controls. Left/right selects an action; OK
  activates it. Up opens the scrubber. Back closes the controls; Back with the
  controls hidden stops playback. Audio/subtitle menus use the same controls.
- Options contains video mode, theme music, navigation taps, autoplay, bitrate,
  the 4:3 filter, server selection and sign-out. The 4:3 filter applies to the
  home rows, search and the alphabetical library view. Movies and episodes are
  judged by their own picture; a show by its first episode, measured once on
  first sight and remembered in the cache folder. An update waiting under
  Options is marked with an amber dot in the menu.
- To leave the app, open MiSTer's OSD and return to the MiSTer Menu core.

### Sound and playback quality

Theme music and navigation taps can be disabled in Options. The default bitrate
is 3 Mbps; try 1.5–2 Mbps if playback stalls. The 4.5 and 6 Mbps choices are
experimental and may cause video or audio stalls. This setting is a request
ceiling, not a measured stream rate.

### Update, rollback and remove

Open **Options > Updates** for available versions, release notes and download
sizes. Checks run in the background at startup and every six hours while browsing.
Failed background checks stay quiet. Manual checks show errors; previously fetched
release information remains available offline.

Public builds notify about newer public releases. Early-access notifications are
off by default but can be enabled in Updates. Beta builds notify about newer betas
and public releases that catch up. Older public versions remain an explicit channel
choice. Dismiss a release notification from its detail page. Nothing installs or
switches channels automatically.

If a beta needs a code you have not saved, choose **Enter code and update**, or
**Install for browsing**. Your current version will keep working. Codes and older
receipts survive updates and rollback. Downloading continues if Plex closes;
activation waits for **Restart now**. **Later** leaves the current version active.
If startup fails, the previous selection is restored.

An ordinary Downloader run can refresh `misterzine-plex-downloads/package.zip`;
it cannot activate that package. The selected channel is registered in
`downloader_misterzine_plex.ini`. Installation data stays in
`/media/fat/misterzine-plex`.

**MisterZine-Plex-Rollback** selects the previous installed release without opening
Plex. Return to MiSTer Menu before using it. **MisterZine-Plex-Uninstall** works
offline and removes Plex binaries, entries, staging files and its database
registration. The default keeps sign-in, preferences, artwork and acquired codes.
Removing all Plex data requires typing **REMOVE**. Downloader and unrelated files
are preserved. A manually extracted `misterzine-plex-beta` ZIP folder can be
removed separately after installation.

### Showcase captures

Options ends with the app version and build information. Select Version and
press OK three times in quick succession (less than two seconds between presses)
to toggle Showcase Mode. A brief message confirms whether it is on or off.
Showcase Mode uses generic library names and hides library totals and numeric
library position counts. Titles, artwork, episode details and watched/progress
indicators remain visible. The account name is also omitted from Sign out.
The mode lasts until the app exits; repeat the shortcut to turn it off sooner.
After switching, Back returns to Home so cached menu images cannot expose old
labels. This is capture styling, not full account anonymization; server selection
and sign-in screens can still contain identifying information.

## Display setup


CRT output is intended primarily for NTSC 15 kHz CRT output at 480i. Component
and Y/C profiles enforce 480i. This is profile detection, not cable or display
detection: RGB-only CRT profiles need **Safe 480i** in the core OSD.

The core OSD has **Video output: App settings / Safe 480i**. Normally leave it
on App settings. HDMI 480p is selected in the app's Options and requires a fresh
two-second OK hold to keep the change. It reverts if you do not confirm.

For HDMI scaling and aspect settings, see [HDMI setup](HDMI.md).
PAL is not yet supported. RGB-only CRTs are not detected automatically.

## MisterZine Plex Core - Patreon beta access

Official beta builds let anyone link Plex, browse libraries, search, change
settings and use watched-state controls. Starting or resuming a movie or episode
requires the member code for that release. Browsing theme music remains available.
Public and normal development builds play without a code. The FPGA core and
separate playback tools do not check Patreon access.

### Unlocking a beta

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

### Older file-key builds

If your older release post supplies a patron key ZIP, extract it onto the SD
card root, preserving `misterzine-plex/beta-keys/<batch>.key`, then press Play.
Keep older keys for rollback. Numeric-code builds do not convert or delete them.

### Does access expire?

There is no automatic membership check, expiry or device binding. Cancellation
does not disable an acquired build. A later release may require a new code.
Existing component licenses and modification rights remain unchanged.
