// Package beta gates official early-access builds with an offline patron file.
// This is a convenience gate, not protection against modified application builds.
package beta

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// These values are set together by the release builder using -ldflags -X.
// A normal development/public build has neither value and is unlocked.
var Batch string
var KeySHA256 string

var batchName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

var ErrLocked = errors.New("Patreon beta key needed. See the beta access guide.")
var ErrBuild = errors.New("Beta access setup is invalid. Reinstall this release.")

// Check re-reads the file each time so installing a key needs no app restart.
// One file per batch preserves access to older builds when a new key is added.
func Check(settingsDir string) error {
	if Batch == "" && KeySHA256 == "" {
		return nil
	}
	want, err := hex.DecodeString(KeySHA256)
	if !batchName.MatchString(Batch) || err != nil || len(want) != sha256.Size {
		return ErrBuild
	}
	f, err := os.Open(filepath.Join(settingsDir, "beta-keys", Batch+".key"))
	if err != nil {
		return ErrLocked
	}
	defer f.Close()
	key, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil || len(key) != 32 {
		return ErrLocked
	}
	sum := sha256.Sum256(key)
	if hex.EncodeToString(sum[:]) != KeySHA256 {
		return ErrLocked
	}
	return nil
}
