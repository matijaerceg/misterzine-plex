package beta

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func codeBuild(t *testing.T) string {
	t.Helper()
	oldChannel, oldBatch, oldCode, oldKey := Channel, Batch, CodeSHA256, KeySHA256
	t.Cleanup(func() { Channel, Batch, CodeSHA256, KeySHA256 = oldChannel, oldBatch, oldCode, oldKey })
	sum := sha256.Sum256([]byte("012345"))
	Channel, Batch, CodeSHA256, KeySHA256 = "beta", "fixture", hex.EncodeToString(sum[:]), ""
	return t.TempDir()
}

func TestCodeLifecycle(t *testing.T) {
	dir := codeBuild(t)
	if Check(dir) != ErrLocked {
		t.Fatal("missing receipt unlocked")
	}
	for _, code := range []string{"12345", "0123450", "abcdef", "012346", " 012345", "012345\n"} {
		if Unlock(dir, code) != ErrLocked {
			t.Fatalf("accepted invalid input %q", code)
		}
	}
	if err := Unlock(dir, "012345"); err != nil {
		t.Fatal(err)
	}
	if Check(dir) != nil {
		t.Fatal("receipt did not persist")
	}
	raw, err := os.ReadFile(receiptPath(dir))
	if err != nil || string(raw) != "unlocked\n" {
		t.Fatal("unexpected receipt", err)
	}
	original := CodeSHA256
	sum := sha256.Sum256([]byte("654321"))
	CodeSHA256 = hex.EncodeToString(sum[:])
	if Check(dir) != ErrLocked {
		t.Fatal("new verifier reused old receipt")
	}
	if err := Unlock(dir, "654321"); err != nil {
		t.Fatal(err)
	}
	CodeSHA256 = original
	if Check(dir) != nil {
		t.Fatal("rollback lost access")
	}
	Batch = "next"
	if Check(dir) != ErrLocked {
		t.Fatal("different batch reused receipt")
	}
	Batch = "fixture"
	if err := Unlock(dir, "012345"); err != nil {
		t.Fatal("same batch update", err)
	}
	if err := os.WriteFile(receiptPath(dir), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if Check(dir) != ErrLocked {
		t.Fatal("damaged receipt unlocked")
	}
	if err := Unlock(dir, "012345"); err != nil {
		t.Fatal("could not repair receipt", err)
	}
}

func TestCodeCannotSave(t *testing.T) {
	dir := codeBuild(t)
	if err := os.WriteFile(filepath.Join(dir, "beta-unlocks"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Unlock(dir, "012345"); err == nil || err == ErrLocked {
		t.Fatal("missing storage error", err)
	}
	if Check(dir) != ErrLocked {
		t.Fatal("failed save unlocked")
	}
}

func TestCodeBuildMetadata(t *testing.T) {
	dir := codeBuild(t)
	hash := CodeSHA256
	for _, tc := range []struct{ channel, batch, code, key string }{
		{"beta", "fixture", "", ""}, {"beta", "", hash, ""}, {"beta", "../bad", hash, ""},
		{"beta", "fixture", "invalid", ""}, {"beta", "fixture", hash, hash},
		{"public", "fixture", hash, ""}, {"development", "fixture", hash, ""}, {"", "fixture", hash, ""}, {"unknown", "", "", ""},
	} {
		Channel, Batch, CodeSHA256, KeySHA256 = tc.channel, tc.batch, tc.code, tc.key
		if Check(dir) != ErrBuild {
			t.Fatalf("invalid metadata accepted: %+v", tc)
		}
	}
	for _, channel := range []string{"", "public", "development"} {
		Channel, Batch, CodeSHA256, KeySHA256 = channel, "", "", ""
		if Check(dir) != nil || IsBeta() {
			t.Fatal("public/development gate or badge")
		}
	}
}
