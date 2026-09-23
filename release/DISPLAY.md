# Display setup

[Getting started](README.md) · [Using Plex](USING.md) · [Display setup](DISPLAY.md) · [Troubleshooting](TROUBLESHOOTING.md)

This beta is intended primarily for NTSC 15 kHz CRT output at 480i. Component
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

PAL is not yet supported. HDMI full-height scaling refinements are deferred;
some settings may leave borders. Automatic detection of RGB-only CRTs is not
included. For playback stalls, see [troubleshooting](TROUBLESHOOTING.md).
