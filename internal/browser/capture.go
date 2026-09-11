package browser

import (
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/mxschmitt/playwright-go"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	resourceDocument = "document"
	resourceFetch    = "fetch"
	resourceXHR      = "xhr"

	headerAuthorization = "authorization"
	headerFetchMode     = "sec-fetch-mode"
	headerUserAgent     = "user-agent"
	fetchModeCORS       = "cors"

	scoreCORS = 1
	scoreAuth = 2
)

type scoredHeaders struct {
	headers []session.Header
	score   int
}

type headerRecorder struct {
	doc   atomic.Pointer[[]session.Header]
	fetch atomic.Pointer[scoredHeaders]
}

func recordHeaders(page playwright.Page) *headerRecorder {
	r := &headerRecorder{}
	page.OnRequest(r.record)

	return r
}

func (r *headerRecorder) record(req playwright.Request) {
	u, err := url.Parse(req.URL())
	if err != nil || !strings.HasSuffix(u.Hostname(), redditDomain) {
		return
	}

	headers, err := headersOf(req)
	if err != nil {
		return
	}

	switch req.ResourceType() {
	case resourceDocument:
		if u.Hostname() == wwwHost && req.Method() == "GET" {
			r.doc.Store(&headers)
		}
	case resourceFetch, resourceXHR:
		r.offerFetch(&scoredHeaders{headers: headers, score: fetchScore(headers)})
	}
}

func (r *headerRecorder) offerFetch(candidate *scoredHeaders) {
	for {
		current := r.fetch.Load()
		if current != nil && candidate.score <= current.score {
			return
		}

		if r.fetch.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func (r *headerRecorder) snapshot() (doc, fetch []session.Header) {
	if d := r.doc.Load(); d != nil {
		doc = *d
	}

	if f := r.fetch.Load(); f != nil {
		fetch = f.headers
	}

	return doc, fetch
}

func (r *headerRecorder) complete() bool {
	fetch := r.fetch.Load()

	return r.doc.Load() != nil && fetch != nil && fetch.score >= scoreCORS
}

func headersOf(req playwright.Request) ([]session.Header, error) {
	hs, err := req.HeadersArray()
	if err != nil {
		return nil, err
	}

	headers := make([]session.Header, 0, len(hs))
	for _, h := range hs {
		headers = append(headers, session.Header{Name: strings.ToLower(h.Name), Value: h.Value})
	}

	return headers, nil
}

func fetchScore(headers []session.Header) int {
	score := 0

	if headerValue(headers, headerFetchMode) == fetchModeCORS {
		score += scoreCORS
	}

	if hasHeader(headers, headerAuthorization) {
		score += scoreAuth
	}

	return score
}

func hasHeader(hs []session.Header, name string) bool {
	return headerValue(hs, name) != ""
}

func headerValue(hs []session.Header, name string) string {
	for _, h := range hs {
		if h.Name == name {
			return h.Value
		}
	}

	return ""
}
