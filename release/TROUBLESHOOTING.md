# Troubleshooting

[Getting started](README.md) · [Using Plex](USING.md) · [Display setup](DISPLAY.md)

## Common problems

| Problem | What to try |
| --- | --- |
| Installation fails | Check networking, free space and current MiSTer Linux, then rerun the installer. Missing dependencies are reported before loading the core. |
| Plex sign-in code expired | Request a new code on the sign-in screen. |
| Server moved or is unreachable | Open the drawer with Back, then **Options > Choose server again**. You can also sign out there. |
| Browsing works but Play asks for a code | Enter the member code for this version. See [beta access](BETA_ACCESS.md). This is separate from Plex sign-in. |
| Playback stalls | Try 1.5–2 Mbps in Options and check that the server can transcode the media. The 4.5 and 6 Mbps choices are experimental. |
| CRT mode or HDMI proportions look wrong | Check [display setup](DISPLAY.md), especially Safe 480i for RGB-only CRT profiles. |
| Settings appear reset | A valid backup is restored when possible; otherwise the app asks you to sign in again. The damaged file is preserved on the card. |
| New release will not start | Startup recovery restores the previous selection. Return to MiSTer Menu and use **MisterZine-Plex-Rollback** if needed. |

## Known beta limitations

- PAL is not yet supported; some HDMI scaling settings may leave borders.
- RGB-only CRT profiles require manually selecting Safe 480i.
- Experimental higher bitrates can stall. Start at the default 3 Mbps.
- The artwork/theme cache has no disk quota; monitor SD card space.

## Report a bug

[Open a bug report](https://github.com/matijaerceg/misterzine-plex-core/issues/new?template=bug_report.yml).
Include the app version/release ID, MiSTer hardware and system version,
cable/output profile, whether the Plex server is local or remote, and steps
with the expected and actual result. Version information is in Options.

For diagnostics, return to MiSTer Menu and run **MisterZine-Plex-Diagnostics**.
It writes `/media/fat/misterzine-plex/diagnostics.json`.
Review it before sharing: media titles and playback details can appear.
Never share `plexcrt.json`, its backups, `.plextoken`, member codes, beta keys,
unlock receipts or an unreviewed card image. Remove account/server identifiers
from screenshots and reports.
