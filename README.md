# MisterZine Plex Core

Browse and watch your Plex library on MiSTer FPGA, with an interface designed
for CRTs and controlled from your gamepad. Supports search, playback controls,
NTSC 480i and HDMI 480p.

![MisterZine Plex Core library browsing screen](docs/images/plex-browsing.png)

**First beta:** anyone can install, link Plex and browse. Video playback in
official beta builds requires a six-digit Patreon member code for that release.
[How beta access works](release/BETA_ACCESS.md).

## Get started

You need a MiSTer, a controller or keyboard, a network connection and a Plex
account with access to a server capable of transcoding your media. Use current
MiSTer Linux with Python 3.9 or newer and allow at least 250 MB free, plus cache
and retained releases. Check each release's notes for tested hardware.

1. Download `MisterZine-Plex-Install.sh` from
   [GitHub Releases](https://github.com/matijaerceg/misterzine-plex-core/releases)
   when a build is available, and copy it into your SD card's `Scripts` folder.
2. Run **Scripts > MisterZine-Plex-Install** and confirm the release.
3. Link your account at [plex.tv/link](https://plex.tv/link), choose your server,
   then enter the member code when you first press Play.

[Full setup instructions](release/README.md) cover display configuration and
manual installation. Update All is optional. Launch again from
**Scripts > MisterZine-Plex-Run**. Older builds may have different script names;
follow the instructions included with your download.

## Before you try it

This is beta software. Playback and watched/unwatched controls update your real
Plex account; start with a small test library. PAL is not yet supported.
RGB-only CRT profiles need **Safe 480i** in the core OSD. Higher experimental
bitrates can stall, and some HDMI scaling settings may leave borders.

## Guides and help

- [Using Plex](release/USING.md): controls, settings, updates, rollback and removal.
- [Display setup](release/DISPLAY.md): CRT profiles, HDMI and aspect ratio.
- [Troubleshooting and bug reports](release/TROUBLESHOOTING.md).
- [Build from source](docs/BUILD.md), [architecture](docs/ARCHITECTURE.md)
  and [contributing](CONTRIBUTING.md).

[Application terms](release/TERMS.md) and
[component licenses](release/THIRD_PARTY_NOTICES.md) apply separately; source
visibility does not grant additional application rights. Movie artwork retains
its owners' rights. Not affiliated with or endorsed by Plex or the MiSTer project.
