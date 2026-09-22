package plex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ThemeFile fetches a show's theme without putting credentials in a URL or
// a subprocess argument. Cancelling navigation also cancels the HTTP request.
func (c *Client) ThemeFile(ctx context.Context, key, path string) (string, error) {
	if path == "" {
		data, err := c.themeGet(ctx, "/library/metadata/"+key)
		if err != nil {
			return "", err
		}
		var meta struct {
			Items []xmlItem `xml:"Directory"`
		}
		if err = xml.Unmarshal(data, &meta); err != nil {
			return "", err
		}
		if len(meta.Items) > 0 {
			path = meta.Items[0].Theme
		}
	}
	if !strings.HasPrefix(path, "/library/metadata/") || strings.Contains(path, "?") {
		return "", errors.New("no theme")
	}
	sum := sha256.Sum256([]byte(c.Host + "|" + path))
	file := filepath.Join(c.CacheDir, "theme-"+hex.EncodeToString(sum[:])+".mp3")
	if st, err := os.Stat(file); err == nil && st.Size() > 0 {
		return file, nil
	}
	data, err := c.themeGet(ctx, path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty theme")
	}
	f, err := os.CreateTemp(c.CacheDir, ".theme-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = os.Rename(tmp, file); err != nil {
		return "", err
	}
	return file, nil
}

func (c *Client) themeGet(ctx context.Context, path string) ([]byte, error) {
	r, err := c.req(path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(r.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("theme unavailable")
	}
	const limit = 16 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if len(data) > limit {
		return nil, errors.New("theme too large")
	}
	return data, err
}
