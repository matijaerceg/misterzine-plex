# Help

[Downloads](https://github.com/matijaerceg/misterzine-plex-core/releases) · [Installation](https://github.com/matijaerceg/misterzine-plex-core#install)

### Before you start

This is unstable beta software. It talks to your real Plex account: playback,
watched marks and resume points change your library, and a bug could change
them wrongly. Use it at your own risk; no responsibility is taken for what it
does to your library or your MiSTer.

### How do I use it?

D-pad moves, OK selects, and Back returns or opens the menu. During playback,
OK opens the controls; Back closes them. Press Back with the controls hidden to
stop. To leave the app, use MiSTer's on-screen menu to return to the Menu core.

### Why is it asking for another code?

The Plex sign-in code links your account. Early-access releases also need a
six-digit code from that version's Patreon post to play videos. Enter it when
you press Play, or under **Options > Beta access**. It's saved for next time.
Public releases are free and need no playback code.

### Installation or sign-in isn't working

Check the network connection, SD card space and that MiSTer Linux is current
(Python 3.9 or newer). Rerun the installer to repair files; settings are kept.
For an expired Plex sign-in code, request a new one on the sign-in screen.
For a missing server, try **Options > Choose server again**.
Older builds may have different script names; follow their included instructions.

### Playback stutters

Try 1.5 or 2 Mbps in Options, and check that your Plex server can transcode the
video. The default is 3 Mbps; higher settings are experimental.

### The picture looks wrong

CRT output uses NTSC 480i. For RGB-only CRT profiles, choose **Safe 480i** in the
core's on-screen menu. PAL isn't supported yet. Select HDMI 480p in Options;
hold OK for two seconds to keep the change, or let it revert.
For HDMI, leave **Scale** on **Normal**. If the picture is cropped or too small,
see [HDMI setup](https://github.com/matijaerceg/misterzine-plex-core/blob/main/docs/HDMI.md).

### How do I update or remove it?

Use **Options > Updates** to install a release, then choose **Restart now** when
ready. Sign-in and settings are kept. If you need to go back, return to MiSTer
Menu and run **MisterZine-Plex-Rollback**.

Downloads go through MiSTer Downloader, and fall back to a direct fetch of the
release file when that fails. If both fail, the message under **Options > Updates**
says whether your MiSTer's connection, its clock, or the release is the problem.
A board whose clock is far off cannot verify any secure site, so set the time in
MiSTer Menu or let it sync over the network. The full reasons are kept in
`/media/fat/misterzine-plex/updates/last-error.log` and in the diagnostics file.

To remove it, return to MiSTer Menu and run **MisterZine-Plex-Uninstall**.
It keeps your settings by default; removing all Plex data requires typing REMOVE.

### Still stuck?

[Report a problem](https://github.com/matijaerceg/misterzine-plex-core/issues/new?template=bug_report.yml)
with the version from Options, your MiSTer model, display connection and what
happened. If needed, **MisterZine-Plex-Diagnostics** writes a diagnostics file in
`/media/fat/misterzine-plex/`. Run it from MiSTer Menu and review the file before
sharing; it can contain media titles. Don't share account files, tokens or member codes.
