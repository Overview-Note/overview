// Package update checks the project's published GitHub releases for a newer
// version. It only reports the result; nothing is downloaded or installed.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ReleasesURL is the GitHub endpoint queried for the latest public release.
const ReleasesURL = "https://api.github.com/repos/Overview-Note/overview/releases/latest"

// requestTimeout bounds a single release lookup so a hung network cannot stall
// the caller indefinitely.
const requestTimeout = 10 * time.Second

// UserAgent identifies Overview to the GitHub API, which rejects requests
// without one.
const UserAgent = "Overview"

// release is the subset of the GitHub release payload this package reads.
type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckLatest fetches the latest published release and compares its tag with
// currentVersion. When a newer version exists hasUpdate is true and url points
// at the release page on GitHub.
func CheckLatest(ctx context.Context, currentVersion string) (latest, url string, hasUpdate bool, err error) {
	return checkLatest(ctx, http.DefaultClient, ReleasesURL, currentVersion)
}

func checkLatest(ctx context.Context, client *http.Client, endpoint, currentVersion string) (latest, url string, hasUpdate bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", UserAgent+"/"+strings.TrimSpace(currentVersion))

	resp, err := client.Do(req)
	if err != nil {
		return "", "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("github releases: unexpected status %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", false, err
	}
	latest = strings.TrimSpace(rel.TagName)
	url = strings.TrimSpace(rel.HTMLURL)
	if latest == "" {
		return "", url, false, fmt.Errorf("github releases: response has no tag_name")
	}
	return latest, url, Compare(currentVersion, latest) < 0, nil
}

// Compare orders two semantic versions, returning -1, 0 or 1. A leading "v" or
// "V" is ignored and any pre-release/build suffix is discarded. A version that
// cannot be parsed sorts below every parseable one.
func Compare(a, b string) int {
	av, aok := parse(a)
	bv, bok := parse(b)
	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return -1
	case !bok:
		return 1
	}
	for i := range av {
		switch {
		case av[i] < bv[i]:
			return -1
		case av[i] > bv[i]:
			return 1
		}
	}
	return 0
}

// parse turns "v1.2.3-beta.1" into its numeric components. Missing components
// are treated as zero; non-numeric ones make the version unparseable.
func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return out, false
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
