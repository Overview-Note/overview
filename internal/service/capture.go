package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

const (
	captureUserAgent   = "OverviewBot/1.0"
	captureTimeout     = 10 * time.Second
	captureMaxBytes    = 2 << 20
	captureMaxText     = 5000
	captureMaxRedirect = 5
)

// ErrCaptureFetchFailed indicates the remote page could not be retrieved. The
// HTTP adapter maps it to 502 Bad Gateway.
var ErrCaptureFetchFailed = errors.New("failed to fetch url")

// allowPrivateNetworks disables the SSRF address checks. It exists only so the
// unit tests can point the fetcher at an httptest server on loopback; the
// production path never enables it.
var allowPrivateNetworks = false

// Preview is the metadata extracted from a remote page.
type Preview struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	URL   string `json:"url"`
}

// FetchPreview downloads rawURL and extracts a title plus a plain-text summary.
// It refuses to connect to loopback, private, link-local or metadata addresses
// so a signed-in user cannot use it to probe the internal network.
func FetchPreview(ctx context.Context, rawURL string) (Preview, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Preview{}, core.Invalidf("invalid url")
	}
	if !allowPrivateNetworks && isPrivateHost(u.Hostname()) {
		return Preview{}, core.Invalidf("url points to a private address")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Preview{}, core.Invalidf("invalid url")
	}
	req.Header.Set("User-Agent", captureUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := newCaptureClient().Do(req)
	if err != nil {
		return Preview{}, fmt.Errorf("%w: %v", ErrCaptureFetchFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return Preview{}, fmt.Errorf("%w: status %d", ErrCaptureFetchFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, captureMaxBytes))
	if err != nil {
		return Preview{}, fmt.Errorf("%w: %v", ErrCaptureFetchFailed, err)
	}
	preview := extractPreview(string(body))
	preview.URL = u.String()
	if preview.Title == "" {
		preview.Title = u.Hostname()
	}
	return preview, nil
}

// newCaptureClient builds a client whose dialer re-checks every resolved
// address at connection time (defeating DNS rebinding) and whose redirects are
// bounded and re-validated.
func newCaptureClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !allowPrivateNetworks {
		dialer.Control = func(_ string, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || isPrivateIP(ip) {
				return fmt.Errorf("%w: blocked address %s", ErrCaptureFetchFailed, host)
			}
			return nil
		}
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 8 * time.Second,
			ExpectContinueTimeout: time.Second,
			DisableKeepAlives:     true,
		},
		Timeout: captureTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= captureMaxRedirect {
				return fmt.Errorf("%w: too many redirects", ErrCaptureFetchFailed)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: unsupported redirect scheme", ErrCaptureFetchFailed)
			}
			if !allowPrivateNetworks && isPrivateHost(req.URL.Hostname()) {
				return fmt.Errorf("%w: redirect to private address", ErrCaptureFetchFailed)
			}
			return nil
		},
	}
}

// isPrivateHost reports whether a URL host (domain or literal IP) is definitely
// non-public. Hostnames are left to the dialer's connection-time check.
func isPrivateHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return isPrivateIP(ip)
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// CGNAT (100.64/10), benchmarking (198.18/15) and "this network"
		// (192.0.0/24) are not publicly routable.
		if ip4[0] == 100 && ip4[1]&0xc0 == 0x40 {
			return true
		}
		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0 {
			return true
		}
	}
	// IPv6 unique local addresses (fc00::/7).
	if len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc {
		return true
	}
	return false
}

var (
	metaTagRe  = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	metaAttrRe = regexp.MustCompile(`(?is)\b(property|name|content)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	titleTagRe = regexp.MustCompile(`(?is)<title\b[^>]*>([\s\S]*?)</title>`)
	// RE2 has no backreferences, so each element is stripped separately.
	dropBlockRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<head\b[^>]*>.*?</head\s*>`),
		regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`),
		regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`),
		regexp.MustCompile(`(?is)<noscript\b[^>]*>.*?</noscript\s*>`),
		regexp.MustCompile(`(?is)<template\b[^>]*>.*?</template\s*>`),
		regexp.MustCompile(`(?is)<svg\b[^>]*>.*?</svg\s*>`),
	}
	commentRe    = regexp.MustCompile(`(?s)<!--.*?-->`)
	tagRe        = regexp.MustCompile(`(?s)<[^>]*>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// extractPreview pulls a title and plain-text body out of an HTML document.
// The <title>/og:title pair wins for the title and the stripped body wins for
// the text, with the meta descriptions as a fallback.
func extractPreview(page string) Preview {
	title := cleanText(metaValue(page, "property", "og:title"))
	if title == "" {
		if m := titleTagRe.FindStringSubmatch(page); m != nil {
			title = cleanText(m[1])
		}
	}

	text := bodyText(page)
	if text == "" {
		text = cleanText(metaValue(page, "property", "og:description"))
	}
	if text == "" {
		text = cleanText(metaValue(page, "name", "description"))
	}
	return Preview{Title: title, Text: truncateRunes(text, captureMaxText)}
}

func bodyText(page string) string {
	cleaned := page
	for _, re := range dropBlockRes {
		cleaned = re.ReplaceAllString(cleaned, " ")
	}
	cleaned = commentRe.ReplaceAllString(cleaned, " ")
	cleaned = tagRe.ReplaceAllString(cleaned, " ")
	return cleanText(cleaned)
}

func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

// metaValue returns the content of the first <meta> tag whose property/name
// attribute equals want.
func metaValue(page, attr, want string) string {
	for _, tag := range metaTagRe.FindAllString(page, -1) {
		attrs := map[string]string{}
		for _, m := range metaAttrRe.FindAllStringSubmatch(tag, -1) {
			value := m[2]
			if value == "" {
				value = m[3]
			}
			if value == "" {
				value = m[4]
			}
			attrs[strings.ToLower(m[1])] = value
		}
		if strings.EqualFold(attrs[attr], want) {
			return attrs["content"]
		}
	}
	return ""
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
