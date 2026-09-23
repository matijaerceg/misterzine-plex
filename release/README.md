# Getting started

[Overview](https://github.com/matijaerceg/misterzine-plex-core) · [Using Plex](USING.md) · [Display setup](DISPLAY.md) · [Troubleshooting](TROUBLESHOOTING.md)

These instructions describe the first beta installer and member-code flow.
Use the instructions included with your download if you have an older build.
Downloads appear on the [GitHub releases page](https://github.com/matijaerceg/misterzine-plex-core/releases) when published.

## Before you start

You need a MiSTer FPGA, a controller or keyboard, a network connection, and a
Plex account with access to a server capable of transcoding your chosen media.
Use current MiSTer Linux with Python 3.9 or newer. Allow at least 250 MB free,
plus space for cached artwork and retained releases. Check the release notes
for the hardware and display combinations tested for that build.

**Beta playback requires a Patreon member code.** Anyone can install, link Plex
and browse. Your Plex sign-in code and your beta member code are different:
one links your Plex account; the other unlocks video playback in this beta.
See [beta access](BETA_ACCESS.md) for details.

For an RGB-only CRT profile, select **Safe 480i** in the core OSD before playback.
Read [display setup](DISPLAY.md) for CRT and HDMI configuration. PAL is not yet
supported. Playback and watched/unwatched controls update your real Plex account;
start with a small test library.

## Install and sign in

1. Download `MisterZine-Plex-Install.sh` from the chosen
   [GitHub release](https://github.com/matijaerceg/misterzine-plex-core/releases)
   and copy it into the SD card's `Scripts` folder.
2. Run **Scripts > MisterZine-Plex-Install** and confirm the offered release.
   The installer locates MiSTer Downloader or downloads a verified official copy,
   verifies the package and decoder, and opens Plex. Update All is optional.
3. Enter the displayed Plex sign-in code at [plex.tv/link](https://plex.tv/link),
   then choose your server on MiSTer.
4. Select a movie or episode and press Play. Enter the six-digit member code
   from that version's Patreon post to unlock beta playback.

Next time, launch **Scripts > MisterZine-Plex-Run**. Use the Scripts entry:
loading an RBF directly does not start the application.

The installer runs only the Plex database and preserves other Downloader
settings. It does not change your MiSTer INI files or the separate MisterZine
Arcade Frontend. Repeating installation repairs files without resetting settings.
Public releases, when available, do not need a beta code; the installer offers
public by default when published and explains paid access when offering a beta.

## Manual ZIP installation

Extract the release ZIP onto the card root and run its
`Scripts/MisterZine-Plex-Install.sh`. It installs the included package from
`misterzine-plex-alpha` (a retained folder name, even for beta builds).
The first decoder download still requires a connection.

## Next steps

- [Using Plex](USING.md): controls, settings, updates and removal.
- [Display setup](DISPLAY.md): CRT profiles, HDMI and aspect ratio.
- [Troubleshooting](TROUBLESHOOTING.md): installation, playback and bug reports.

See [application terms](TERMS.md), [component notices](THIRD_PARTY_NOTICES.md)
and the supplied `licenses` folder. Matching core and presenter sources are
supplied in `corresponding-source.zip`.
