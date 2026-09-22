package ui

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Helpers exercise real pipes/process ownership without opening the sound device.
func TestThemeProcess(t *testing.T) {
	role := os.Getenv("PLEX_THEME_HELPER")
	if role == "" {
		return
	}
	if role == "decode" {
		buf := make([]byte, 1920)
		for i := 0; i < len(buf); i += 2 {
			binary.LittleEndian.PutUint16(buf[i:], 20000)
		}
		for {
			if _, err := os.Stdout.Write(buf); err != nil {
				os.Exit(0)
			}
		}
	}
	if role == "stalled" {
		time.Sleep(time.Hour)
	}
	if role == "audio" {
		f, err := os.Create(os.Getenv("PLEX_THEME_CAPTURE"))
		if err != nil {
			os.Exit(2)
		}
		buf := make([]byte, 1920)
		for {
			n, err := io.ReadFull(os.Stdin, buf)
			f.Write(buf[:n])
			if err != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		f.Close()
		os.Exit(0)
	}
}

func TestThemeFadeAndRelease(t *testing.T) {
	for _, role := range []string{"decode", "stalled"} {
		t.Run(role, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "pcm")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			var decode, output *exec.Cmd
			go func() {
				defer close(done)
				playThemeCommands(ctx, func(hard context.Context) (*exec.Cmd, *exec.Cmd) {
					makeCmd := func(role string) *exec.Cmd {
						cmd := exec.CommandContext(hard, os.Args[0], "-test.run=^TestThemeProcess$")
						cmd.Env = append(os.Environ(), "PLEX_THEME_HELPER="+role, "PLEX_THEME_CAPTURE="+file)
						return cmd
					}
					decode, output = makeCmd(role), makeCmd("audio")
					return decode, output
				})
			}()
			time.Sleep(600 * time.Millisecond)
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("theme did not release audio")
			}
			if decode.ProcessState == nil || output.ProcessState == nil {
				t.Fatal("child was not reaped")
			}
			if role == "decode" {
				pcm, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if len(pcm) < 48000 {
					t.Fatalf("too little PCM: %d", len(pcm))
				}
				peak := int16(0)
				for i := 0; i < len(pcm); i += 4 {
					v := int16(binary.LittleEndian.Uint16(pcm[i:]))
					if v > peak {
						peak = v
					}
					if binary.LittleEndian.Uint16(pcm[i:]) != binary.LittleEndian.Uint16(pcm[i+2:]) {
						t.Fatal("stereo gain mismatch")
					}
				}
				if peak < 5400 || peak > 5500 {
					t.Fatalf("gain peak=%d", peak)
				}
				if binary.LittleEndian.Uint16(pcm[len(pcm)-2:]) != 0 {
					t.Fatal("fade did not reach silence")
				}
			}
		})
	}
}
