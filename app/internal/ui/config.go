package ui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config is the app's settings and sign-in, kept as JSON on the SD card.
// The tokens live only here and in memory; nothing logs them.
type Config struct {
	EarlyAccessUpdates bool     `json:"early_access_updates"`
	DismissedUpdates   []string `json:"dismissed_updates,omitempty"`
	RecentSearches     []string `json:"recent_searches,omitempty"`
	NoTaps             bool     `json:"no_taps"`
	NoTheme            bool     `json:"no_theme"`
	Progressive        bool     `json:"progressive"`           // confirmed HDMI 480p preference
	FourThree          bool     `json:"only_4x3"`              // hide media wider than 4:3
	NoAutoplay         bool     `json:"no_autoplay"`           // do not run on to the next episode
	Bitrate            int      `json:"bitrate"`               // transcode cap in kbit/s; 0 is the default
	AudioBoost         int      `json:"audio_boost,omitempty"` // gain when the server folds surround to stereo; 0 is the default
	ClientID           string   `json:"client_id"`
	Token              string   `json:"token"` // the plex.tv account token
	AccountName        string   `json:"account_name,omitempty"`
	ServerURL          string   `json:"server_url"`   // the chosen server
	ServerToken        string   `json:"server_token"` // the token for that server
	ServerName         string   `json:"server_name"`
	LoadWarning        string   `json:"-"`
	LoadError          error    `json:"-"`
	path               string
}

// LoadConfig reads the config file; a missing file gives the defaults. A
// client identifier is made on first run and kept.
func LoadConfig(path string) *Config {
	c := &Config{path: path}
	data, err := os.ReadFile(path)
	needsSave := err != nil
	if err == nil {
		if json.Unmarshal(data, c) != nil {
			// Preserve damaged input before recovering; never silently overwrite it.
			if err := os.Rename(path, path+".corrupt-"+time.Now().Format("20060102-150405")); err != nil {
				c.LoadError = fmt.Errorf("cannot preserve damaged settings: %w", err)
				return c
			}
			c = &Config{path: path, LoadWarning: "Settings were damaged. Please sign in again."}
			if backup, e := os.ReadFile(path + ".bak"); e == nil {
				var recovered Config
				if json.Unmarshal(backup, &recovered) == nil && recovered.ClientID != "" {
					recovered.path = path
					recovered.LoadWarning = "Recovered settings from backup."
					c = &recovered
				}
			}
		}
	} else if !os.IsNotExist(err) {
		c.LoadError = fmt.Errorf("cannot read settings: %w", err)
		return c
	}
	if c.ClientID == "" {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			c.LoadError = err
			return c
		}
		c.ClientID = "misterzine-" + hex.EncodeToString(b[:])
		needsSave = true
	}
	if needsSave || c.LoadWarning != "" {
		if e := c.Save(); e != nil {
			c.LoadError = e
		}
	}
	return c
}

// The tested 3 Mbps Plex request produces about 2 Mbps of video. Dense
// 30 fps content may need a lower choice; this is not a universal limit.
const DefaultBitrate = 3000
const MaxBitrate = 6000

// Bitrates are the caps offered, in kbit/s.
var Bitrates = []int{1000, 1500, 2000, 3000, 4500, 6000}

// BitrateKbps is the cap in force.
func (c *Config) BitrateKbps() int {
	if c.Bitrate <= 0 {
		return DefaultBitrate
	}
	return min(c.Bitrate, MaxBitrate)
}

// AudioBoosts are the downmix gains offered, as the Plex transcoder's
// audioBoost value: 100 is unity. They apply only when the server folds a
// surround track to stereo; stereo sources are copied untouched.
var AudioBoosts = []struct {
	Label string
	Value int
}{{"Off", 100}, {"Small", 175}, {"Large", 300}}

const DefaultAudioBoost = 175

// AudioBoostValue is the downmix gain in force.
func (c *Config) AudioBoostValue() int {
	if c.AudioBoost <= 0 {
		return DefaultAudioBoost
	}
	return c.AudioBoost
}

// SignedIn reports whether a server is on record.
func (c *Config) SignedIn() bool { return c.ServerURL != "" && c.ServerToken != "" }

// SignOut forgets the account and the server.
func (c *Config) SignOut() error {
	next := *c
	next.RecentSearches = nil
	next.AccountName = ""
	next.Token, next.ServerURL, next.ServerToken, next.ServerName = "", "", "", ""
	data, _ := json.MarshalIndent(&next, "", "  ")
	// Clear recovery credentials first so a later corrupt primary cannot undo sign-out.
	if err := atomicConfig(c.path+".bak", data); err != nil {
		return err
	}
	if err := atomicConfig(c.path, data); err != nil {
		return err
	}
	*c = next
	return nil
}

func atomicConfig(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Save atomically replaces settings and retains the last valid configuration.
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if old, e := os.ReadFile(c.path); e == nil && json.Valid(old) {
		if e = atomicConfig(c.path+".bak", old); e != nil {
			return e
		}
	}
	return atomicConfig(c.path, data)
}
