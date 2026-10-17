package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const DefaultVersion = "2.26"
const maxManual = 24 << 20
const maxCache = 48 << 20

var versionPattern = regexp.MustCompile(`^2\.[0-9]{1,3}$`)

func ValidVersion(v string) bool { return versionPattern.MatchString(v) }

type Loader struct {
	Version, CacheDir string
	// BaseURL and Client allow deterministic tests without the live website.
	BaseURL string
	Client  *http.Client
}

func (l Loader) Load(ctx context.Context, refresh bool) (*Catalog, error) {
	v := l.Version
	if v == "" {
		v = DefaultVersion
	}
	if !ValidVersion(v) {
		return nil, fmt.Errorf("documentation version must be a LilyPond series such as 2.26")
	}
	dir := l.CacheDir
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("find documentation cache: %w", err)
		}
		dir = filepath.Join(cache, "lyra", "docs")
	}
	path := filepath.Join(dir, "v"+v+"-index-1.json")
	if !refresh {
		if c, err := readCache(path, v); err == nil {
			return c, nil
		}
	}
	base := l.BaseURL
	if base == "" {
		base = "https://lilypond.org/doc/v" + v + "/Documentation/"
	}
	client := l.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	catalog := &Catalog{Version: v}
	for _, manual := range []struct{ name, file string }{{"Notation Reference", "notation"}, {"Learning Manual", "learning"}} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source := strings.TrimRight(base, "/") + "/" + manual.file + "-big-page.html"
		sections, err := download(ctx, client, manual.name, source)
		if err != nil {
			return nil, fmt.Errorf("load LilyPond %s docs: %w; Ctrl+R to retry", v, err)
		}
		catalog.Sections = append(catalog.Sections, sections...)
	}
	catalog.Prepare()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return catalog, fmt.Errorf("documentation loaded, but cache directory failed: %w", err)
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		return nil, err
	}
	if len(data) > maxCache {
		return nil, fmt.Errorf("documentation index exceeds size limit")
	}
	tmp, err := os.CreateTemp(dir, ".index-*")
	if err == nil {
		defer os.Remove(tmp.Name())
		_, err = tmp.Write(data)
		errClose := tmp.Close()
		if err == nil {
			err = errClose
		}
		if err == nil {
			err = os.Rename(tmp.Name(), path)
		}
	}
	if err != nil {
		return catalog, fmt.Errorf("documentation loaded, but could not save offline cache: %w", err)
	}
	return catalog, nil
}

func download(ctx context.Context, client *http.Client, manual, source string) ([]Section, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Lyra documentation reader")
	req.Header.Set("Accept-Language", "en")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", manual, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxManual+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxManual {
		return nil, fmt.Errorf("%s exceeds download limit", manual)
	}
	sections, err := Parse(strings.NewReader(string(data)), manual, source)
	if err != nil {
		return nil, err
	}
	// A proxy error page can be valid HTML with a heading. Do not cache it as
	// the manual; official manuals have hundreds of sections and identify LilyPond.
	if len(sections) < 10 || !strings.Contains(strings.ToLower(string(data[:min(len(data), 20000)])), "lilypond") {
		return nil, fmt.Errorf("%s response is not a LilyPond manual", manual)
	}
	return sections, nil
}

func readCache(path, version string) (*Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCache+1))
	if err != nil || len(data) > maxCache {
		return nil, fmt.Errorf("invalid documentation cache")
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil || c.Version != version || len(c.Sections) == 0 {
		return nil, fmt.Errorf("invalid documentation cache")
	}
	c.Prepare()
	return &c, nil
}
