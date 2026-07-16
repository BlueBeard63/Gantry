package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const modulePath = "github.com/B-Commissions/Gantry"

// checkForUpdate prints a one-line notice when a newer Gantry tag
// exists. It asks the Go module proxy at most once per day (cached),
// spends at most 1.5s on the network, stays silent on any failure,
// and is skipped for local builds (go install ./cmd/gantry reports
// version "(devel)") or when GANTRY_NO_UPDATE_CHECK is set.
func checkForUpdate() {
	if os.Getenv("GANTRY_NO_UPDATE_CHECK") != "" {
		return
	}
	current := cliVersion()
	if current == "" || current == "(devel)" {
		return
	}
	latest := latestVersion()
	if latest == "" || !semverLess(current, latest) {
		return
	}
	fmt.Fprintf(os.Stderr,
		"gantry: %s is available (you have %s) - update with: go install %s/cmd/gantry@latest\n",
		latest, current, modulePath)
}

func cliVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return bi.Main.Version
}

type updateCache struct {
	CheckedAt time.Time `json:"checkedAt"`
	Latest    string    `json:"latest"`
}

// latestVersion returns the newest tagged version, from the daily
// cache when fresh, else from the module proxy.
func latestVersion() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	cachePath := filepath.Join(base, "gantry", "update-check.json")

	var c updateCache
	if data, err := os.ReadFile(cachePath); err == nil {
		if json.Unmarshal(data, &c) == nil && time.Since(c.CheckedAt) < 24*time.Hour {
			return c.Latest
		}
	}

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get("https://proxy.golang.org/" + proxyEscape(modulePath) + "/@latest")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var info struct {
		Version string `json:"Version"`
	}
	if json.NewDecoder(resp.Body).Decode(&info) != nil {
		return ""
	}

	c = updateCache{CheckedAt: time.Now(), Latest: info.Version}
	if data, err := json.Marshal(c); err == nil {
		_ = os.MkdirAll(filepath.Dir(cachePath), 0o700)
		_ = os.WriteFile(cachePath, data, 0o600)
	}
	return info.Version
}

// proxyEscape applies the module proxy's case encoding: an uppercase
// letter becomes '!' + its lowercase form.
func proxyEscape(path string) string {
	var b strings.Builder
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// semverLess reports a < b for vX.Y.Z strings (prerelease suffixes are
// ignored; unparseable versions compare as equal).
func semverLess(a, b string) bool {
	pa, okA := semverParts(a)
	pb, okB := semverParts(b)
	if !okA || !okB {
		return false
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func semverParts(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
