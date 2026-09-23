# Contributing

Start with [help and bug reports](release/README.md).
Maintainers can follow [beta release preparation](docs/BETA_RELEASES.md) and the
[release hardware check](docs/HARDWARE_CHECK.md).

Report bugs with the app version, MiSTer hardware, output cable/profile, local or
remote server, and reproduction steps. Review diagnostics before sharing them;
never attach account files, patron keys or unreviewed card images.

Keep changes focused and explain the resulting behavior. Check the applicable
[component terms](release/THIRD_PARTY_NOTICES.md) and [application terms](release/TERMS.md).
No additional license grant is implied by source availability.

On Linux with Go 1.27 and Python 3:

```sh
cd app
go test ./...
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go vet ./...
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build ./cmd/plexcrt
cd ..
python3 -m unittest discover -s release -p 'test*.py'
python3 tools/check_public_content.py
```

Independent installer and publishing checks are described in
[distribution](release/DISTRIBUTION.md). The real Downloader integration test
uses a disposable `/media` card tree and a supplied official Downloader archive.
`tools/check_update_device.py` exercises the synthetic lifecycle on MiSTer while
preserving the normal installation; read its required fixture arguments first.

Hardware-dependent changes also need a MiSTer smoke test. Test credentials and
media must belong to the tester; playback can update watched state. Keep raw
test journals and session notes in the private notes directory, outside Git.

[Build from source](docs/BUILD.md) · [Architecture](docs/ARCHITECTURE.md) · [Detailed reference](docs/REFERENCE.md)
