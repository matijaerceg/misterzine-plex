package beta

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestAccess(t *testing.T) {
	oldBatch, oldHash := Batch, KeySHA256
	t.Cleanup(func() { Batch, KeySHA256 = oldBatch, oldHash })
	dir := t.TempDir()
	Batch, KeySHA256 = "", ""
	if err := Check(dir); err != nil {
		t.Fatal(err)
	}
	Batch = "2026-09"
	if Check(dir) != ErrBuild {
		t.Fatal("partial build configuration must fail closed")
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	sum := sha256.Sum256(key)
	KeySHA256 = hex.EncodeToString(sum[:])
	if Check(dir) != ErrLocked {
		t.Fatal("missing key unlocked")
	}
	keys := filepath.Join(dir, "beta-keys")
	if err := os.Mkdir(keys, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(keys, Batch+".key")
	for _, bad := range [][]byte{nil, []byte("wrong"), make([]byte, 32), append(append([]byte{}, key...), 0)} {
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if Check(dir) != ErrLocked {
			t.Fatal("invalid key unlocked")
		}
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir); err != nil {
		t.Fatal("newly copied key not accepted:", err)
	}
	Batch = "2026-10"
	if Check(dir) != ErrLocked {
		t.Fatal("old key unlocked new batch")
	}
	Batch = "2026-09"
	if err := Check(dir); err != nil {
		t.Fatal("rollback lost access:", err)
	}
	Batch = "../2026-09"
	if Check(dir) != ErrBuild {
		t.Fatal("invalid batch accepted")
	}
	Batch, KeySHA256 = "", ""
	if err := Check(dir); err != nil {
		t.Fatal("public release not unlocked:", err)
	}
}
