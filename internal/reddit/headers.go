package reddit

import (
	http "github.com/bogdanfinn/fhttp"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	hUserAgent     = "user-agent"
	hAccept        = "accept"
	hAcceptLang    = "accept-language"
	hAcceptEnc     = "accept-encoding"
	hReferer       = "referer"
	hAuthorization = "authorization"
	hContentType   = "content-type"
	hOrigin        = "origin"
	hCookie        = "cookie"
	hFetchDest     = "sec-fetch-dest"
	hFetchMode     = "sec-fetch-mode"
	hFetchSite     = "sec-fetch-site"
	hFetchUser     = "sec-fetch-user"
	hPriority      = "priority"
	hTE            = "te"
	hUpgradeInsec  = "upgrade-insecure-requests"

	acceptAny      = "*/*"
	acceptHTML     = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	acceptLangEN   = "en-US,en;q=0.5"
	acceptEncAll   = "gzip, deflate, br, zstd"
	priorityFetch  = "u=4"
	priorityNav    = "u=0, i"
	teTrailers     = "trailers"
	bearerPrefix   = "Bearer "
	defaultReferer = webBase + "/"
)

var firefoxOrder = []string{
	hUserAgent, hAccept, hAcceptLang, hAcceptEnc,
	hReferer, hAuthorization, hContentType, hOrigin,
	hCookie, hFetchDest, hFetchMode, hFetchSite, hFetchUser,
	hPriority, hTE,
}

var perRequest = map[string]bool{
	"host": true, "connection": true, "content-length": true,
	hCookie: true, hAuthorization: true, hContentType: true,
	hReferer: true, hOrigin: true, hFetchSite: true, hFetchUser: true,
	hUpgradeInsec: true,
}

func defaultFetchHeaders(ua string) []session.Header {
	return []session.Header{
		{Name: hUserAgent, Value: ua},
		{Name: hAccept, Value: acceptAny},
		{Name: hAcceptLang, Value: acceptLangEN},
		{Name: hAcceptEnc, Value: acceptEncAll},
		{Name: hPriority, Value: priorityFetch},
		{Name: hTE, Value: teTrailers},
	}
}

func defaultDocHeaders(ua string) []session.Header {
	return []session.Header{
		{Name: hUserAgent, Value: ua},
		{Name: hAccept, Value: acceptHTML},
		{Name: hAcceptLang, Value: acceptLangEN},
		{Name: hAcceptEnc, Value: acceptEncAll},
		{Name: hPriority, Value: priorityNav},
		{Name: hTE, Value: teTrailers},
	}
}

func (c *Client) apiHeaders(tok string, form bool) http.Header {
	sess := c.session()

	src := sess.FetchHeaders
	if len(src) == 0 {
		src = defaultFetchHeaders(sess.UserAgent)
	}

	extra := map[string]string{
		hReferer:       defaultReferer,
		hOrigin:        webBase,
		hAuthorization: bearerPrefix + tok,
		hFetchDest:     "empty",
		hFetchMode:     "cors",
		hFetchSite:     "same-site",
	}

	if form {
		extra[hContentType] = formContentType
	}

	return c.buildHeaders(src, extra)
}

func (c *Client) docSource() []session.Header {
	sess := c.session()
	if len(sess.DocumentHeaders) > 0 {
		return sess.DocumentHeaders
	}

	return defaultDocHeaders(sess.UserAgent)
}

func (c *Client) docHeaders() http.Header {
	h := c.buildHeaders(c.docSource(), map[string]string{
		hFetchDest:    "document",
		hFetchMode:    "navigate",
		hFetchSite:    "none",
		hFetchUser:    "?1",
		hUpgradeInsec: "1",
	})
	h[http.HeaderOrderKey] = moveBefore(h[http.HeaderOrderKey], hUpgradeInsec, hFetchDest)

	return h
}

func (c *Client) buildHeaders(captured []session.Header, extra map[string]string) http.Header {
	values, seen := mergeHeaders(captured, extra, c.session().UserAgent)

	h := http.Header{}
	order := make([]string, 0, len(values)+1)
	canonical := make(map[string]bool, len(firefoxOrder))

	for _, name := range firefoxOrder {
		canonical[name] = true

		if v := values[name]; v != "" {
			h.Set(name, v)
			order = append(order, name)
		} else if name == hCookie {
			order = append(order, name)
		}
	}

	for _, name := range seen {
		if !canonical[name] && values[name] != "" {
			h.Set(name, values[name])
			order = append(order, name)
		}
	}

	h[http.HeaderOrderKey] = order

	return h
}

func mergeHeaders(captured []session.Header, extra map[string]string, ua string) (map[string]string, []string) {
	values := make(map[string]string, len(captured)+len(extra))
	seen := make([]string, 0, len(captured)+len(extra)+1)

	add := func(name, value string) {
		if _, dup := values[name]; !dup {
			seen = append(seen, name)
		}
		values[name] = value
	}

	for _, h := range captured {
		if perRequest[h.Name] || h.Name[0] == ':' {
			continue
		}
		add(h.Name, h.Value)
	}

	if values[hUserAgent] == "" && ua != "" {
		add(hUserAgent, ua)
	}

	for k, v := range extra {
		add(k, v)
	}

	return values, seen
}

func moveBefore(order []string, name, before string) []string {
	out := make([]string, 0, len(order))

	for _, n := range order {
		if n == name {
			continue
		}
		if n == before {
			out = append(out, name)
		}
		out = append(out, n)
	}

	return out
}
