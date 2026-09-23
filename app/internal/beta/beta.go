// Package beta gates official early-access builds with offline patron codes or legacy key files.
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
// Public/development builds omit access verifiers and are unlocked.
var Channel string
var CodeSHA256 string
var Batch string
var KeySHA256 string

var batchName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

var ErrLocked = errors.New("Patreon beta key needed. See the beta access guide.")
var ErrBuild = errors.New("Beta access setup is invalid. Reinstall this release.")

// Requirement describes either the running build or a prospective update.
// Verifying an update never changes the running build's access configuration.
type Requirement struct {
	Channel    string
	Batch      string
	CodeSHA256 string
	KeySHA256  string
}

func Current() Requirement          { return Requirement{Channel, Batch, CodeSHA256, KeySHA256} }
func IsBeta() bool                  { return Channel == "beta" }
func Check(dir string) error        { return Current().Check(dir) }
func Unlock(dir, code string) error { return Current().Unlock(dir, code) }
func receiptPath(dir string) string { return Current().receiptPath(dir) }

// Forget clears access for all batches, including legacy keys, but keeps settings.
func Forget(dir string) error {
	if dir == "" {
		return errors.New("missing installation directory")
	}
	for _, name := range []string{"beta-unlocks", "beta-keys"} {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// Check re-reads saved access so installing a legacy key needs no restart.
// r.Batch-specific files preserve access to older builds after rotation.
func (r Requirement) Check(settingsDir string) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.CodeSHA256 != "" {
		data, err := os.ReadFile(r.receiptPath(settingsDir))
		if err != nil || string(data) != "unlocked\n" {
			return ErrLocked
		}
		return nil
	}
	if r.Batch == "" && r.KeySHA256 == "" {
		return nil
	}
	want, err := hex.DecodeString(r.KeySHA256)
	if !batchName.MatchString(r.Batch) || err != nil || len(want) != sha256.Size {
		return ErrBuild
	}
	f, err := os.Open(filepath.Join(settingsDir, "beta-keys", r.Batch+".key"))
	if err != nil {
		return ErrLocked
	}
	defer f.Close()
	key, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil || len(key) != 32 {
		return ErrLocked
	}
	sum := sha256.Sum256(key)
	if hex.EncodeToString(sum[:]) != r.KeySHA256 {
		return ErrLocked
	}
	return nil
}

func (r Requirement) Validate() error {
	if r.Channel != "" && r.Channel != "development" && r.Channel != "public" && r.Channel != "beta" {
		return ErrBuild
	}
	if r.CodeSHA256 != "" {
		raw, err := hex.DecodeString(r.CodeSHA256)
		if r.Channel != "beta" || r.KeySHA256 != "" || !batchName.MatchString(r.Batch) || err != nil || len(raw) != sha256.Size || hex.EncodeToString(raw) != r.CodeSHA256 {
			return ErrBuild
		}
		return nil
	}
	if r.Channel == "beta" && (r.Batch == "" || r.KeySHA256 == "") {
		return ErrBuild
	}
	if (r.Channel == "public" || r.Channel == "development") && (r.Batch != "" || r.KeySHA256 != "") {
		return ErrBuild
	}
	return nil
}

func (r Requirement) receiptPath(dir string) string {
	return filepath.Join(dir, "beta-unlocks", r.Batch+"-"+r.CodeSHA256+".receipt")
}

// Unlock verifies exactly six ASCII digits, then persists access before returning.
// Receipts are convenience state, not authentication against modified clients.
func (r Requirement) Unlock(dir, code string) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.CodeSHA256 == "" {
		return ErrBuild
	}
	if len(code) != 6 {
		return ErrLocked
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return ErrLocked
		}
	}
	sum := sha256.Sum256([]byte(code))
	if hex.EncodeToString(sum[:]) != r.CodeSHA256 {
		return ErrLocked
	}
	if r.Check(dir) == nil {
		return nil
	}
	path := r.receiptPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".unlock-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString("unlocked\n"); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
