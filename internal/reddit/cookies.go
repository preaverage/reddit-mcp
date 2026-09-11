package reddit

import (
	"log"
	"net/url"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	cookieDomain = ".reddit.com"
	cookiePath   = "/"
)

var webURL, _ = url.Parse(webBase)

func (c *Client) session() *session.Session {
	return c.sess.Load()
}

func (c *Client) replaceSession(sess *session.Session) {
	c.sess.Store(sess)
	c.loadCookies(sess)
}

func (c *Client) loadCookies(sess *session.Session) {
	cookies := make([]*http.Cookie, 0, len(sess.Cookies))

	for _, ck := range sess.Cookies {
		cookies = append(cookies, toHTTPCookie(ck))
	}

	c.jar.SetCookies(webURL, cookies)
}

func (c *Client) syncCookies() *session.Session {
	next := c.session().Clone()

	for _, hc := range c.jar.Cookies(webURL) {
		next.SetCookie(toSessionCookie(hc))
	}

	c.sess.Store(next)

	return next
}

func (c *Client) token() string {
	for _, ck := range c.jar.Cookies(webURL) {
		if ck.Name == session.CookieToken {
			return ck.Value
		}
	}

	return c.session().Token()
}

func (c *Client) persist() {
	if err := c.syncCookies().Save(); err != nil {
		log.Printf("warning: could not save session: %v", err)
	}
}

func toHTTPCookie(ck session.Cookie) *http.Cookie {
	hc := &http.Cookie{
		Name:     ck.Name,
		Value:    ck.Value,
		Domain:   ck.Domain,
		Path:     ck.Path,
		HttpOnly: ck.HttpOnly,
		Secure:   ck.Secure,
	}

	if ck.Expires > 0 {
		hc.Expires = time.Unix(int64(ck.Expires), 0)
	}

	return hc
}

func toSessionCookie(hc *http.Cookie) session.Cookie {
	ck := session.Cookie{
		Name:     hc.Name,
		Value:    hc.Value,
		Domain:   hc.Domain,
		Path:     hc.Path,
		HttpOnly: hc.HttpOnly,
		Secure:   hc.Secure,
		Expires:  session.NoExpiry,
	}

	if !hc.Expires.IsZero() {
		ck.Expires = float64(hc.Expires.Unix())
	}
	if ck.Domain == "" {
		ck.Domain = cookieDomain
	}
	if ck.Path == "" {
		ck.Path = cookiePath
	}

	return ck
}
