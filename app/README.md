# Application

The Go browser and playback interface for MisterZine Plex Core.
See [build instructions](../docs/BUILD.md), [architecture](../docs/ARCHITECTURE.md)
and [user guide](../release/README.md).

Runtime flags include -config, -cache, -player and -ffmpeg for installation
paths; -version identifies the build and -check validates the installation.
Developer flags include -dump and -cpuprofile. PLEXCRT_NOPLAY permits UI walks
without launching media. PLEXCRT_WAIT_DOT="speed,path" tunes the dot that runs
while a playback starts or stalls (pixels per field, path width in pixels;
default "2,36").
Synthetic controller input uses /tmp/plexcrt.ctl.
