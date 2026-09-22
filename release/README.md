# MisterZine Plex Core — limited alpha

This is an early, independent Plex client for MiSTer. It is not affiliated with
or endorsed by Plex. Expect bugs. Start with a small test library: playback and
the watched/unwatched controls update your real Plex account.

Official Patreon beta builds require a key for playback. See
[Beta access](BETA_ACCESS.md) for installation instructions. Public builds
do not require a key.

## Install

1. Use a current MiSTer Linux installation, a network connection, and a Plex
   account with access to a server capable of transcoding the chosen media.
2. Extract this ZIP onto the root of your MiSTer SD card. It adds an installer
   under `Scripts` and a `misterzine-plex-alpha` package folder.
3. On MiSTer, run **Scripts > MisterZine-Plex-Core-Install**. The first installation
   downloads and verifies the pinned FFmpeg decoder. Allow about 150 MB free,
   plus space for cached artwork and future releases.
4. Run **Scripts > MisterZine-Plex-Core-Run**. Use the displayed code at
   **plex.tv/link**, then choose your server. Settings are saved locally.

The Scripts entry loads the matching core and app together. Loading an RBF
directly does not start the app. This alpha has its own Scripts entry; it does
not modify the separate MisterZine launcher or your MiSTer INI files.

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

Return to the MiSTer Menu first. Extract the next package and run its installer.
It verifies the new files before selecting the new release. Your account,
settings and cache remain in `/media/fat/misterzine-plex`.

**MisterZine-Plex-Core-Rollback** selects the previous installed release. It is
available after your first update; it leaves current settings in place.

**MisterZine-Plex-Core-Remove** disables this app's Scripts entries. It preserves the
account, cache and binaries so you can restore them by running the installer.
For complete removal, sign out first, then remove the `misterzine-plex` and
`misterzine-plex-alpha` folders using your usual SD card file manager.

## If something goes wrong

- Sign-in expired: request a new code on the sign-in screen.
- Server moved or unreachable: Back opens the drawer; choose Options, then
  **Choose server again**. You can also sign out there.
- Installation error: check card space, networking and MiSTer Linux, then rerun
  the installer. Missing dependencies are reported before loading the core.
- Damaged settings: a valid backup is restored when possible; otherwise the
  app asks you to sign in again. The damaged file is preserved on the card.
- Return to MiSTer Menu, then run **MisterZine-Plex-Core-Diagnostics**. It writes
  `/media/fat/misterzine-plex/diagnostics.json`. Review it before sharing: media
  titles and playback details can appear. Never share `plexcrt.json`, its
  backups, `.plextoken`, or an unreviewed card image.

For a useful report, include the release ID, DE10 or MiSTer Pi, MiSTer version,
cable/output profile, local or remote server, steps and expected/actual result.
The artwork/theme cache has no disk quota yet; monitor SD card space.

## Quick hardware test

On each unit: install and launch from Scripts; link and select a server; browse;
play, pause, seek and stop one item; check theme/taps; leave to MiSTer Menu and
relaunch. Confirm the active CRT profile stays at 480i. On HDMI, test the 480p
hold-to-confirm and timeout-to-revert once. Check normal startup has no test card.

See `THIRD_PARTY_NOTICES.md` and the `licenses` folder for retained licenses.
Matching core and presenter sources are supplied in `corresponding-source.zip`.
