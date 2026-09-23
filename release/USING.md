# Using Plex

[Getting started](README.md) · [Using Plex](USING.md) · [Display setup](DISPLAY.md) · [Troubleshooting](TROUBLESHOOTING.md)

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

## Sound and playback quality

Theme music and navigation taps can be disabled in Options. The default bitrate
is 3 Mbps; try 1.5–2 Mbps if playback stalls. The 4.5 and 6 Mbps choices are
experimental and may cause video or audio stalls. This setting is a request
ceiling, not a measured stream rate.

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

## Showcase captures

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
