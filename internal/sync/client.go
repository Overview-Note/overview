// Package sync implements the desktop vault synchronization engine. It mirrors
// a local Overview vault against a remote Overview server over HTTP, using note
// ids from frontmatter as the stable identity and content versions as the
// three-way merge base.
package sync

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

// Sync sentinel errors. They let the engine decide between retrying, pausing
// and reporting.
var (
	ErrUnauthorized = errors.New("sync: unauthorized")
	ErrForbidden    = errors.New("sync: forbidden")
	ErrConflict     = errors.New("sync: conflict")
	ErrNotFound     = errors.New("sync: not found")
	ErrNotSupported = errors.New("sync: not supported")
	ErrTooLarge     = errors.New("sync: payload too large")
	ErrServer       = errors.New("sync: server error")
)

// HTTPError carries a non-success status returned by the remote.
type HTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("remote returned %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("remote returned %d", e.Status)
}

func (e *HTTPError) Unwrap() error {
	switch e.Status {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusConflict:
		return ErrConflict
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusNotImplemented:
		return ErrNotSupported
	case http.StatusRequestEntityTooLarge:
		return ErrTooLarge
	default:
		if e.Status >= 500 {
			return ErrServer
		}
		return nil
	}
}

// ClientOptions tunes a RemoteClient.
type ClientOptions struct {
	// Token is the API bearer token. It is never logged.
	Token string
	// UserAgent identifies the client.
	UserAgent string
	// Timeout bounds a single request. Defaults to 30s.
	Timeout time.Duration
	// HTTPClient overrides the transport (tests).
	HTTPClient *http.Client
}

// RemoteClient talks to an Overview server's sync API.
type RemoteClient struct {
	baseURL string
	token   string
	ua      string
	http    *http.Client
	// stream is used for the long-lived SSE subscription. It must not carry a
	// whole-request timeout, which would tear down an idle stream.
	stream *http.Client
}

// NewRemoteClient validates the server URL and builds a client. Plain HTTP is
// accepted only for loopback hosts so a token can never travel in the clear to
// a remote host.
func NewRemoteClient(serverURL string, opts ClientOptions) (*RemoteClient, error) {
	base, err := normalizeServerURL(serverURL)
	if err != nil {
		return nil, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := opts.HTTPClient
	var stream *http.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
		stream = &http.Client{}
	} else {
		stream = client
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "Overview-Sync/1.0"
	}
	return &RemoteClient{baseURL: base, token: opts.Token, ua: ua, http: client, stream: stream}, nil
}

// normalizeServerURL validates the scheme and host and returns the URL without
// a trailing slash.
func normalizeServerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", core.Invalidf("server URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", core.Invalidf("invalid server URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", core.Invalidf("server URL must use http or https")
	}
	if u.Host == "" {
		return "", core.Invalidf("server URL must include a host")
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return "", core.Invalidf("plain http is only allowed for loopback hosts")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// isLoopbackHost reports whether host names the local machine.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// Version returns the remote vault id from the manifest without applying any
// change. It is used to validate connectivity.
func (c *RemoteClient) manifestURL() string { return c.baseURL + "/api/v1/sync/manifest" }

func (c *RemoteClient) do(ctx context.Context, method, url string, body io.Reader, contentType string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

// statusError builds an HTTPError from a response, draining and closing the
// body. The body is never included beyond the server-provided message.
func statusError(resp *http.Response) error {
	defer resp.Body.Close()
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload)
	return &HTTPError{Status: resp.StatusCode, Code: payload.Error.Code, Message: payload.Error.Message}
}

// Manifest fetches the vault inventory. When etag matches the server's current
// validator the second result is true and the manifest is nil.
func (c *RemoteClient) Manifest(ctx context.Context, etag string) (*core.VaultManifest, bool, error) {
	headers := map[string]string{"Cache-Control": "no-cache"}
	if etag != "" {
		headers["If-None-Match"] = `"` + etag + `"`
	}
	resp, err := c.do(ctx, http.MethodGet, c.manifestURL(), nil, "", headers)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode == http.StatusNotModified {
		resp.Body.Close()
		return nil, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, statusError(resp)
	}
	defer resp.Body.Close()
	var manifest core.VaultManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, false, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.ETag == "" {
		manifest.ETag = trimETag(resp.Header.Get("ETag"))
	}
	return &manifest, false, nil
}

// Validate checks connectivity by fetching the manifest without using a
// conditional request.
func (c *RemoteClient) Validate(ctx context.Context) (string, error) {
	manifest, _, err := c.Manifest(ctx, "")
	if err != nil {
		return "", err
	}
	return manifest.VaultID, nil
}

// Changes fetches the incremental change feed after since (note sequence,
// exclusive) and sinceTs (tombstone timestamp, RFC3339, exclusive). limit caps
// the returned note changes; it may be zero to use the server default.
func (c *RemoteClient) Changes(ctx context.Context, since int64, sinceTs string, limit int) (core.SyncChanges, error) {
	q := url.Values{}
	q.Set("since", strconv.FormatInt(since, 10))
	if sinceTs != "" {
		q.Set("sinceTs", sinceTs)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	resp, err := c.do(ctx, http.MethodGet, c.baseURL+"/api/v1/sync/changes?"+q.Encode(), nil, "", nil)
	if err != nil {
		return core.SyncChanges{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return core.SyncChanges{}, statusError(resp)
	}
	defer resp.Body.Close()
	var out core.SyncChanges
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.SyncChanges{}, fmt.Errorf("decode changes: %w", err)
	}
	return out, nil
}

// sseBackoffBase and sseBackoffMax bound the reconnect delay of Events.
const (
	sseBackoffBase = 500 * time.Millisecond
	sseBackoffMax  = 30 * time.Second
)

// Events subscribes to the server's change stream and invokes onEvent for every
// change event, passing the current note sequence and tombstone timestamp. It
// ignores comment heartbeats and reconnects automatically with exponential
// backoff until ctx is cancelled. Permanent auth failures are returned so the
// caller can stop retrying.
func (c *RemoteClient) Events(ctx context.Context, onEvent func(latestSeq int, latestTs string)) error {
	delay := sseBackoffBase
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		connected, err := c.streamEvents(ctx, onEvent)
		if connected {
			delay = sseBackoffBase
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && (errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay < sseBackoffMax {
			delay *= 2
			if delay > sseBackoffMax {
				delay = sseBackoffMax
			}
		}
	}
}

// streamEvents performs a single SSE connection. It returns connected=true once
// the server accepted the stream, and returns when the stream ends, the server
// errors, or ctx is cancelled.
func (c *RemoteClient) streamEvents(ctx context.Context, onEvent func(latestSeq int, latestTs string)) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/sync/events", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.stream.Do(req)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, statusError(resp)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var event, data string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return true, nil
			}
			return true, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if event == "change" && data != "" {
				var payload struct {
					LatestSeq int64  `json:"latestSeq"`
					LatestTs  string `json:"latestTs"`
				}
				if json.Unmarshal([]byte(data), &payload) == nil {
					onEvent(int(payload.LatestSeq), payload.LatestTs)
				}
			}
			event, data = "", ""
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			chunk := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data != "" {
				data += "\n"
			}
			data += chunk
		}
	}
}

// GetNoteRaw fetches the complete Markdown bytes and version of a note.
func (c *RemoteClient) GetNoteRaw(ctx context.Context, path string) ([]byte, string, error) {
	u := c.baseURL + "/api/v1/note?path=" + url.QueryEscape(path) + "&raw=1"
	resp, err := c.do(ctx, http.MethodGet, u, nil, "", map[string]string{"Accept": "text/markdown"})
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", statusError(resp)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return raw, trimETag(resp.Header.Get("ETag")), nil
}

// NoteWriteResult is the remote note state after a raw write.
type NoteWriteResult struct {
	Path    string `json:"path"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

// PutNoteRaw writes complete Markdown bytes at path using the baseVersion
// contract. The caller must pass a non-empty baseVersion ("*" to create).
func (c *RemoteClient) PutNoteRaw(ctx context.Context, path string, raw []byte, baseVersion string) (NoteWriteResult, error) {
	if baseVersion == "" {
		return NoteWriteResult{}, core.Invalidf("baseVersion is required")
	}
	body := map[string]any{
		"path":        path,
		"raw":         true,
		"content":     string(raw),
		"baseVersion": baseVersion,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return NoteWriteResult{}, err
	}
	resp, err := c.do(ctx, http.MethodPut, c.baseURL+"/api/v1/note", bytes.NewReader(encoded), "application/json", nil)
	if err != nil {
		return NoteWriteResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return NoteWriteResult{}, statusError(resp)
	}
	defer resp.Body.Close()
	var note NoteWriteResult
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		return NoteWriteResult{}, fmt.Errorf("decode note write: %w", err)
	}
	if note.Version == "" {
		note.Version = trimETag(resp.Header.Get("ETag"))
	}
	return note, nil
}

// DeleteNote moves a remote note to the server trash.
func (c *RemoteClient) DeleteNote(ctx context.Context, path string) error {
	u := c.baseURL + "/api/v1/note?path=" + url.QueryEscape(path)
	resp, err := c.do(ctx, http.MethodDelete, u, nil, "", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return statusError(resp)
	}
	resp.Body.Close()
	return nil
}

// Rename moves a remote note or folder.
func (c *RemoteClient) Rename(ctx context.Context, from, to string) error {
	return c.postJSON(ctx, "/api/v1/rename", map[string]string{"from": from, "to": to})
}

// Mkdir creates a remote folder.
func (c *RemoteClient) Mkdir(ctx context.Context, path string) error {
	return c.postJSON(ctx, "/api/v1/folder", map[string]string{"path": path})
}

func (c *RemoteClient) postJSON(ctx context.Context, endpoint string, body any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodPost, c.baseURL+endpoint, bytes.NewReader(encoded), "application/json", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return statusError(resp)
	}
	resp.Body.Close()
	return nil
}

// PutAsset writes an attachment at an explicit vault-relative path.
func (c *RemoteClient) PutAsset(ctx context.Context, rel string, r io.Reader) error {
	resp, err := c.do(ctx, http.MethodPut, c.baseURL+"/api/v1/assets/"+escapeRelPath(rel), r, "application/octet-stream", nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return statusError(resp)
	}
	resp.Body.Close()
	return nil
}

// GetAsset opens an attachment stream. The caller closes the returned body.
func (c *RemoteClient) GetAsset(ctx context.Context, rel string) (io.ReadCloser, error) {
	resp, err := c.do(ctx, http.MethodGet, c.baseURL+"/assets/"+escapeRelPath(rel), nil, "", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	return resp.Body, nil
}

// escapeRelPath escapes each segment of a slash-separated path while keeping
// the separators, so nested asset paths stay reachable.
func escapeRelPath(rel string) string {
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func trimETag(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "W/")
	return strings.Trim(v, `"`)
}
