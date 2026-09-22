# Application

The Go browser and playback interface for MisterZine Plex Core.
See [build instructions](../docs/BUILD.md), [architecture](../docs/ARCHITECTURE.md)
and [user guide](../release/README.md).

Runtime flags include -config, -cache, -player and -ffmpeg for installation
paths; -version identifies the build and -check validates the installation.
Developer flags include -dump and -cpuprofile. PLEXCRT_NOPLAY permits UI walks
without launching media. Synthetic controller input uses /tmp/plexcrt.ctl.
