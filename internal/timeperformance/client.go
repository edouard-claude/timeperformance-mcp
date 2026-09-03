// Package timeperformance is a client for the TimePerformance API v4
// (https://pma.timeperformance.com/apidoc/).
package timeperformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the SaaS instance; override with TIMEPERFORMANCE_URL for
// an on-premise deployment.
const DefaultBaseURL = "https://pma.timeperformance.com"

// apiPrefix is appended to the base URL for every call.
const apiPrefix = "/api/v4"

// whoamiID extracts the numeric id from the sentence /whoami returns.
var whoamiID = regexp.MustCompile(`\(id:\s*(\d+)\)`)

// Client is an authenticated TimePerformance API client.
//
// The API allows one request per second and a single in-flight request per
// user (extra calls get a 429 with Retry-After), so every call goes through a
// mutex that both serializes and paces them.
type Client struct {
	baseURL  string
	login    string
	password string
	http     *http.Client

	mu       sync.Mutex
	next     time.Time
	interval time.Duration

	// cached reference data
	usersOnce sync.Once
	users     []User
	usersErr  error
	whoamiVal string
}

// NewClient builds a client from the environment:
//
//	TIMEPERFORMANCE_URL       base URL (optional, defaults to the SaaS instance)
//	TIMEPERFORMANCE_LOGIN     API credential id     (required)
//	TIMEPERFORMANCE_PASSWORD  API credential secret (required)
//
// TP_URL / TP_LOGIN / TP_PASSWORD are accepted as short aliases.
func NewClient() (*Client, error) {
	base := firstEnv("TIMEPERFORMANCE_URL", "TP_URL")
	if base == "" {
		base = DefaultBaseURL
	}
	login := firstEnv("TIMEPERFORMANCE_LOGIN", "TP_LOGIN")
	password := firstEnv("TIMEPERFORMANCE_PASSWORD", "TP_PASSWORD")
	if login == "" || password == "" {
		return nil, fmt.Errorf("TIMEPERFORMANCE_LOGIN and TIMEPERFORMANCE_PASSWORD are required (API credentials from the web app: profile > Settings for user credentials, Administration > API BackOffice for back-office ones)")
	}

	interval := time.Second
	if v := firstEnv("TIMEPERFORMANCE_MIN_INTERVAL_MS", "TP_MIN_INTERVAL_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			interval = time.Duration(ms) * time.Millisecond
		}
	}

	return &Client{
		baseURL:  strings.TrimRight(base, "/"),
		login:    login,
		password: password,
		http:     &http.Client{Timeout: 60 * time.Second},
		interval: interval,
	}, nil
}

// BaseURL returns the instance base URL (without the /api/v4 prefix).
func (c *Client) BaseURL() string { return c.baseURL }

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := strings.TrimSpace(os.Getenv(n)); v != "" {
			return v
		}
	}
	return ""
}

// --- HTTP ---

// do performs one API call, honouring the rate limits. It returns the response
// body, or an error carrying the server's own message.
func (c *Client) do(method, path string, params url.Values, payload any) ([]byte, error) {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode payload: %w", err)
		}
	}

	u := c.baseURL + apiPrefix + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	// One in-flight request at a time, spaced by c.interval.
	c.mu.Lock()
	defer c.mu.Unlock()

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if d := time.Until(c.next); d > 0 {
			time.Sleep(d)
		}

		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, u, reader)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.login, c.password)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		c.next = time.Now().Add(c.interval)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, path, err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("%s %s: read body: %w", method, path, readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			// Rate limited: honour Retry-After (seconds), default 1s.
			wait := time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && secs > 0 {
					wait = time.Duration(secs) * time.Second
				}
			}
			c.next = time.Now().Add(wait)
			lastErr = fmt.Errorf("%s %s: HTTP 429 (rate limited)", method, path)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}
		return nil, httpError(method, path, resp.StatusCode, respBody)
	}
	return nil, lastErr
}

// httpError turns a non-2xx response into an error the caller can act on.
func httpError(method, path string, code int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if len(msg) > 800 {
		msg = msg[:800] + "…"
	}
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s %s: HTTP 401 — credentials rejected. Check TIMEPERFORMANCE_LOGIN / TIMEPERFORMANCE_PASSWORD (never a personal email/password: use the API credentials generated in the web app). %s", method, path, msg)
	case http.StatusForbidden:
		if method == http.MethodGet {
			return fmt.Errorf("%s %s: HTTP 403 — these credentials may not read this resource (the account lacks the permission, or the data is restricted such as cost rates). %s", method, path, msg)
		}
		return fmt.Errorf("%s %s: HTTP 403 — this write was refused. User credentials are read-only (GET only); writes need back-office credentials (Administration > API BackOffice). %s", method, path, msg)
	case http.StatusNotFound:
		return fmt.Errorf("%s %s: HTTP 404 — not found. %s", method, path, msg)
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("%s %s: HTTP 422 — the API rejected the payload: %s", method, path, msg)
	}
	return fmt.Errorf("%s %s: HTTP %d: %s", method, path, code, msg)
}

// get decodes a GET response into out.
func (c *Client) get(path string, params url.Values, out any) error {
	body, err := c.do(http.MethodGet, path, params, nil)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("GET %s: decode response: %w", path, err)
	}
	return nil
}

// GetRaw returns the raw JSON of a GET, for the report endpoints whose payload
// is a deep, report-specific tree not worth mirroring in Go types.
func (c *Client) GetRaw(path string, params url.Values) (json.RawMessage, error) {
	body, err := c.do(http.MethodGet, path, params, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

// send performs a write call and decodes the response into out (may be nil).
func (c *Client) send(method, path string, params url.Values, payload, out any) error {
	body, err := c.do(method, path, params, payload)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	return nil
}

// --- Identity ---

// WhoAmI returns the identity of the authenticated caller.
func (c *Client) WhoAmI() (string, error) {
	if c.whoamiVal != "" {
		return c.whoamiVal, nil
	}
	body, err := c.do(http.MethodGet, "/whoami", nil, nil)
	if err != nil {
		return "", err
	}
	var who string
	if err := json.Unmarshal(body, &who); err != nil {
		who = strings.Trim(strings.TrimSpace(string(body)), `"`)
	}
	c.whoamiVal = who
	return who, nil
}

// --- Resolvers ---

// ResolveProjectID accepts a numeric id or a project name (case-insensitive).
func (c *Client) ResolveProjectID(ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, fmt.Errorf("a project id or name is required")
	}
	if id, err := strconv.Atoi(ref); err == nil {
		return id, nil
	}
	var ids []int
	if err := c.get("/projects/getIdFromName", url.Values{"name": {ref}}, &ids); err != nil {
		return 0, err
	}
	switch len(ids) {
	case 0:
		return 0, fmt.Errorf("no project named %q (list_projects shows the available names)", ref)
	case 1:
		return ids[0], nil
	default:
		return 0, fmt.Errorf("%d projects are named %q (ids %v) — pass the numeric id", len(ids), ref, ids)
	}
}

// ResolvePortfolioID accepts a numeric id or a portfolio name.
func (c *Client) ResolvePortfolioID(ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, fmt.Errorf("a portfolio id or name is required")
	}
	if id, err := strconv.Atoi(ref); err == nil {
		return id, nil
	}
	var ids []int
	if err := c.get("/portfolios/getIdFromName", url.Values{"name": {ref}}, &ids); err != nil {
		return 0, err
	}
	switch len(ids) {
	case 0:
		return 0, fmt.Errorf("no portfolio named %q (list_portfolios shows the available names)", ref)
	case 1:
		return ids[0], nil
	default:
		return 0, fmt.Errorf("%d portfolios are named %q (ids %v) — pass the numeric id", len(ids), ref, ids)
	}
}

// cachedUsers loads the user directory once per process.
func (c *Client) cachedUsers() ([]User, error) {
	c.usersOnce.Do(func() {
		c.users, c.usersErr = c.ListUsers()
	})
	return c.users, c.usersErr
}

// ResolveUserID accepts a numeric id, "me" (the authenticated caller), a full
// name, a login or an email address.
func (c *Client) ResolveUserID(ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, fmt.Errorf("a user id, name or email is required")
	}
	if id, err := strconv.Atoi(ref); err == nil {
		return id, nil
	}
	if strings.EqualFold(ref, "me") {
		who, err := c.WhoAmI()
		if err != nil {
			return 0, err
		}
		// /whoami answers with a human sentence, e.g.
		// `Jane Doe (id: 4242) @ tenant`: the id is the reliable part.
		if m := whoamiID.FindStringSubmatch(who); m != nil {
			return strconv.Atoi(m[1])
		}
		ref = who
	}

	users, err := c.cachedUsers()
	if err != nil {
		return 0, fmt.Errorf("cannot resolve user %q: %w", ref, err)
	}
	var matches []User
	for _, u := range users {
		if strings.EqualFold(u.Name, ref) || strings.EqualFold(u.Email, ref) ||
			strings.EqualFold(strings.TrimSpace(u.Firstname+" "+u.Lastname), ref) ||
			strings.EqualFold(u.ExternalID, ref) {
			matches = append(matches, u)
		}
	}
	if len(matches) == 0 {
		// last resort: substring match, useful for "me" resolving to a login
		lower := strings.ToLower(ref)
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.Name), lower) || strings.Contains(strings.ToLower(u.Email), lower) {
				matches = append(matches, u)
			}
		}
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no user matching %q (list_users shows the directory)", ref)
	case 1:
		return matches[0].ID, nil
	default:
		names := make([]string, 0, len(matches))
		for _, u := range matches {
			names = append(names, fmt.Sprintf("%s (#%d)", u.Name, u.ID))
		}
		return 0, fmt.Errorf("%q matches several users: %s — pass the numeric id", ref, strings.Join(names, ", "))
	}
}
