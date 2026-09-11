package reddit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	apiBase = "https://oauth.reddit.com"
	webBase = "https://www.reddit.com"

	requestTimeoutSeconds = 30
	tokenSlack            = time.Minute
	maxDiscard            = 1 << 20
	errorBodyLimit        = 300

	paramRawJSON = "raw_json"

	formContentType = "application/x-www-form-urlencoded"
)

var (
	tlsProfile = profiles.Firefox_148

	errNoReauth = errors.New("token expired and no browser re-auth available; run `reddit-mcp login`")
)

type Reauth func(ctx context.Context) (*session.Session, error)

type Client struct {
	sess atomic.Pointer[session.Session]

	http tls_client.HttpClient
	jar  tls_client.CookieJar

	reauth    Reauth
	refreshMu sync.Mutex
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	b := strings.TrimSpace(e.Body)
	cut := min(len(b), errorBodyLimit)

	suffix := ""
	if cut < len(b) {
		suffix = "..."
	}

	return fmt.Sprintf("reddit returned HTTP %d: %s%s", e.Status, b[:cut], suffix)
}

func New(sess *session.Session, reauth Reauth) (*Client, error) {
	if sess == nil || sess.Token() == "" {
		return nil, session.ErrNoSession
	}

	jar := tls_client.NewCookieJar()

	hc, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithClientProfile(tlsProfile),
		tls_client.WithCookieJar(jar),
		tls_client.WithTimeoutSeconds(requestTimeoutSeconds),
		tls_client.WithNotFollowRedirects(),
	)
	if err != nil {
		return nil, fmt.Errorf("create tls client: %w", err)
	}

	c := &Client{http: hc, jar: jar, reauth: reauth}
	c.replaceSession(sess)

	return c, nil
}

func (c *Client) Session() *session.Session {
	return c.syncCookies().Clone()
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set(paramRawJSON, "1")

	return c.request(ctx, http.MethodGet, apiBase+path+"?"+query.Encode(), nil)
}

// PostForm submits a form-encoded body. Reddit answers write endpoints with a
// json envelope that carries its own error list, so callers check that too.
func (c *Client) PostForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	if form == nil {
		form = url.Values{}
	}
	form.Set(paramAPIType, "json")
	form.Set(paramRawJSON, "1")

	return c.request(ctx, http.MethodPost, apiBase+path, []byte(form.Encode()))
}

func (c *Client) request(ctx context.Context, method, full string, body []byte) ([]byte, error) {
	out, status, err := c.do(ctx, method, full, body)
	if err != nil {
		return nil, err
	}

	if c.staleToken(status) {
		if rerr := c.refresh(ctx, c.token()); rerr != nil {
			return nil, fmt.Errorf("%w (token refresh failed: %v)", &APIError{status, string(out)}, rerr)
		}

		out, status, err = c.do(ctx, method, full, body)
		if err != nil {
			return nil, err
		}
	}

	if status < 200 || status >= 300 {
		return nil, &APIError{status, string(out)}
	}

	return out, nil
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, v any) (json.RawMessage, error) {
	body, err := c.Get(ctx, path, query)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return body, nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return nil, fmt.Errorf("decode %s: %w (body starts: %.120s)", path, err, bytes.TrimSpace(body))
	}

	return body, nil
}

func (c *Client) FetchHome(ctx context.Context) (int, error) {
	return c.fetchHome(ctx)
}

// staleToken decides whether a rejection is worth a token refresh. Reddit
// sends 403 for ordinary permission problems too, such as editing someone
// else's comment, and retrying those only buries the real reason.
func (c *Client) staleToken(status int) bool {
	switch status {
	case http.StatusUnauthorized:
		return true
	case http.StatusForbidden:
		return !tokenUsable(c.token())
	default:
		return false
	}
}

func (c *Client) do(ctx context.Context, method, full string, body []byte) ([]byte, int, error) {
	tok, err := c.validToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return nil, 0, err
	}

	req.Header = c.apiHeaders(tok, body != nil)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request %s: %w", full, err)
	}
	defer resp.Body.Close()

	out, err := readBody(resp)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read %s: %w", full, err)
	}

	c.syncCookies()

	return out, resp.StatusCode, nil
}

func (c *Client) validToken(ctx context.Context) (string, error) {
	tok := c.token()
	if tokenUsable(tok) {
		return tok, nil
	}

	if err := c.refresh(ctx, tok); err != nil {
		return "", err
	}

	return c.token(), nil
}

func tokenUsable(tok string) bool {
	return tok != "" && time.Until(session.JWTExpiry(tok)) > tokenSlack
}

func (c *Client) refresh(ctx context.Context, stale string) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	if tok := c.token(); tok != stale && tokenUsable(tok) {
		return nil
	}

	if _, err := c.fetchHome(ctx); err != nil {
		log.Printf("token rotation request failed: %v", err)
	}

	if tok := c.token(); tok != stale && tokenUsable(tok) {
		c.persist()
		return nil
	}

	if c.reauth == nil {
		return errNoReauth
	}

	log.Println("token rotation over HTTP failed, falling back to headless browser refresh")

	fresh, err := c.reauth(ctx)
	if err != nil {
		return fmt.Errorf("browser re-auth: %w", err)
	}

	c.replaceSession(fresh)
	c.persist()

	return nil
}

func (c *Client) fetchHome(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, webBase+"/", nil)
	if err != nil {
		return 0, err
	}

	req.Header = c.docHeaders()

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request %s: %w", webBase, err)
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, io.LimitReader(resp.Body, maxDiscard))
	c.syncCookies()

	return resp.StatusCode, nil
}
