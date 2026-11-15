// Package update checks GitHub for a newer persadomapicli release.
package update

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/persado/martechapi-cli/src/pkg/version"
)

// LatestURL redirects to the tag page of the latest release.
const LatestURL = "https://github.com/persado/martechapi-cli/releases/latest"

// Install commands printed in the upgrade notice.
const (
	InstallUnix    = "curl -fsSL https://raw.githubusercontent.com/persado/martechapi-cli/main/install.sh | bash"
	InstallWindows = "irm https://raw.githubusercontent.com/persado/martechapi-cli/main/install.ps1 | iex"
)

// Latest returns the latest release version ("0.1.2"), read from the tag in the
// redirect that releasesURL answers with. It avoids the GitHub REST API, whose
// unauthenticated limit is 60 requests per hour per IP address.
func Latest(ctx context.Context, client *http.Client, releasesURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, releasesURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "persadomapicli/"+version.Version)

	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noFollow.Do(req)
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return "", fmt.Errorf("query latest release: status %d, want a redirect", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return "", fmt.Errorf("parse redirect location: %w", err)
	}
	tag := path.Base(loc.Path)
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("redirect %q does not end in a vX.Y.Z tag", loc)
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// Newer reports whether latest is a higher MAJOR.MINOR.PATCH than current.
// It returns false when either version does not parse.
func Newer(current, latest string) bool {
	c, okC := parse(current)
	l, okL := parse(latest)
	if !okC || !okL {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// InstallCommand returns the upgrade one-liner for the operating system goos.
func InstallCommand(goos string) string {
	if goos == "windows" {
		return InstallWindows
	}
	return InstallUnix
}
