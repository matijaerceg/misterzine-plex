# MisterZine Plex Core

This is an early, independent Plex client for MiSTer. It is not affiliated with
or endorsed by Plex. Expect bugs. Start with a small test library: playback and
the watched/unwatched controls update your real Plex account.

Official Patreon beta builds require a six-digit member code for playback. See
[Beta access](BETA_ACCESS.md) for unlocking instructions. Public builds
do not require a code.

## Install

1. Use a current MiSTer Linux installation, a network connection, and a Plex
   account with access to a server capable of transcoding the chosen media.
2. From the GitHub release, choose `MisterZine-Plex-Install-Public.sh` for the latest
   public release, `MisterZine-Plex-Install-Beta.sh` for the latest early-access
   release, or `MisterZine-Plex-Install-<version>.sh` for that exact version.
   A channel's script is offered only after that channel has a published release.
   Copy the chosen script into the card's `Scripts` folder.
3. Run that script from **Scripts**. It locates MiSTer Downloader (including
   the copy installed by Update All), or downloads a verified official copy if
   needed. Only the Plex database runs. Other databases and settings stay intact.
4. Installation proceeds without a confirmation prompt, including when the Linux
   console is not visible. The filename selects the channel or exact version.
   Early-access playback needs a paid Patreon code. The installer verifies the
   package and decoder, then opens Plex.
5. Use the displayed Plex sign-in code at **plex.tv/link**, then choose your server.
   Subsequently launch **MisterZine Plex Core** from the main menu.

Use current MiSTer Linux with Python 3.9 or newer. Allow at least 250 MB free,
plus artwork cache and retained releases. Download, verification and installation
have separate progress stages. Repeating installation repairs files without
resetting settings. A standalone installer does not require Update All.

For manual ZIP installation, extract the package onto the card root and run its
`Scripts/MisterZine-Plex-Install.sh`. It installs the included package from
`misterzine-plex-alpha`; the first decoder download still requires a connection.

The main-menu entry loads the matching core and app together. Loading an RBF
directly does not start the app. Installation adds a small boot helper to
`linux/user-startup.sh`, preserving other startup commands. It does not modify
the separate MisterZine frontend or your MiSTer INI files. Rollback, diagnostics
and uninstall remain available in Scripts if the app cannot open.

## Controls

- D-pad moves the selection; OK opens it; Back goes back or opens the drawer.
- In the library wall, L/R jumps by sort letter or page. Back selects the view
  tabs, then returns to the previous screen.
- During playback, OK opens the controls. Left/right selects an action; OK
  activates it. Up opens the scrubber. Back closes the controls; Back with the
  controls hidden stops playback. Audio/subtitle menus use the same controls.
- Options contains video mode, theme music, navigation taps, autoplay, bitrate,
  the 4:3 filter, server selection and sign-out.
- To leave the app, open MiSTer's OSD and return to the MiSTer Menu core.

## Video and audio

This alpha is intended primarily for NTSC 15 kHz CRT output at 480i. Component
and Y/C profiles enforce 480i. This is profile detection, not cable or display
detection: RGB-only CRT profiles need **Safe 480i** in the core OSD.

The core OSD has **Video output: App settings / Safe 480i**. Normally leave it
on App settings. HDMI 480p is selected in the app's Options and requires a fresh
two-second OK hold to keep the change. It reverts if you do not confirm.

For pixel inspection, set the core OSD **HDMI aspect** to **Square pixels**.
With 1080p HDMI output and integer vertical scaling (`vscale_mode=1`), the
720x480 app raster occupies a centered 1440x960 area. This preserves square
2x2 pixel blocks and appears wider than the intended CRT proportions. Select
**Original 4:3** to restore the normal shape. The aspect option does not change
analog scan timing.

Theme music plays at half gain. Soft navigation taps follow selection changes;
both can be disabled in Options. Assess their timing on your actual display.

The alpha defaults to 3 Mbps, with 1, 1.5 and 2 Mbps lower choices. The 4.5 and
6 Mbps choices are explicitly marked experimental: they may cause video or
audio stalls. This is a request ceiling, not the measured stream rate; the
tested 3 Mbps request produced about 2 Mbps of video. Try 1.5–2 Mbps if difficult
material stalls. Choices above 6 Mbps are excluded.

PAL and HDMI full-height scaling refinements are deferred. Some HDMI scaling
settings may leave borders. Automatic detection of RGB-only CRTs is not included.

## Update, rollback and remove

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
are preserved. A manually extracted `misterzine-plex-alpha` ZIP folder can be
removed separately after installation.

## If something goes wrong

- Sign-in expired: request a new code on the sign-in screen.
- Server moved or unreachable: Back opens the drawer; choose Options, then
  **Choose server again**. You can also sign out there.
- Installation error: check card space, networking and MiSTer Linux, then rerun
  the installer. Missing dependencies are reported before loading the core.
- Damaged settings: a valid backup is restored when possible; otherwise the
  app asks you to sign in again. The damaged file is preserved on the card.
- Return to MiSTer Menu, then run **MisterZine-Plex-Diagnostics**. It writes
  `/media/fat/misterzine-plex/diagnostics.json`. Review it before sharing: media
  titles and playback details can appear. Never share `plexcrt.json`, its
  backups, `.plextoken`, or an unreviewed card image.

For a useful report, include the release ID, DE10 or MiSTer Pi, MiSTer version,
cable/output profile, local or remote server, steps and expected/actual result.
The artwork/theme cache has no disk quota yet; monitor SD card space.

## Quick hardware test

On each unit: install from Scripts and launch from the main menu; link and select a server; browse;
play, pause, seek and stop one item; check theme/taps; leave to MiSTer Menu and
relaunch. Confirm the active CRT profile stays at 480i. On HDMI, test the 480p
hold-to-confirm and timeout-to-revert once. Check normal startup has no test card.

See `THIRD_PARTY_NOTICES.md` and the `licenses` folder for retained licenses.
Matching core and presenter sources are supplied in `corresponding-source.zip`.

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
