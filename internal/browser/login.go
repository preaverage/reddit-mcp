package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/preaverage/reddit-mcp/internal/session"
)

const (
	loginPoll      = 500 * time.Millisecond
	harvestTimeout = 5 * time.Second
	harvestPoll    = 100 * time.Millisecond
	refreshTimeout = 30 * time.Second
)

var (
	errLoginTimeout   = errors.New("timed out waiting for login")
	errNoNavigation   = errors.New("never observed a navigation request to www.reddit.com")
	errMissingCookies = errors.New("login cookies missing after harvest")
)

var domLoaded = playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}

func Login(ctx context.Context, executable string, timeout time.Duration) (*session.Session, error) {
	b, err := Launch(executable, true)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page, err := b.Page()
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}

	recorder := recordHeaders(page)

	if _, err := page.Goto(loginURL, domLoaded); err != nil {
		return nil, fmt.Errorf("open %s: %w", loginURL, err)
	}

	log.Println("Log in to Reddit in the browser window. Waiting...")

	if err := waitForLogin(ctx, b.Context, timeout); err != nil {
		return nil, err
	}

	log.Println("Login detected, collecting session...")

	return harvestLoggedIn(ctx, b.Context, page, recorder)
}

func Refresh(ctx context.Context, executable string) (*session.Session, error) {
	b, err := Launch(executable, false)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page, err := b.Page()
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}

	recorder := recordHeaders(page)

	if _, err := page.Goto(homeURL, domLoaded); err != nil {
		return nil, fmt.Errorf("open %s: %w", homeURL, err)
	}

	if err := waitForLogin(ctx, b.Context, refreshTimeout); err != nil {
		return nil, fmt.Errorf("browser profile is no longer logged in: %w", err)
	}

	return harvestLoggedIn(ctx, b.Context, page, recorder)
}

func loggedIn(bc playwright.BrowserContext) (bool, error) {
	cookies, err := bc.Cookies(homeURL)
	if err != nil {
		return false, err
	}

	var haveSession, haveToken bool
	for _, c := range cookies {
		switch c.Name {
		case session.CookieSession:
			haveSession = c.Value != ""
		case session.CookieToken:
			haveToken = c.Value != ""
		}
	}

	return haveSession && haveToken, nil
}

func waitForLogin(ctx context.Context, bc playwright.BrowserContext, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		ok, err := loggedIn(bc)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		if time.Now().After(deadline) {
			return errLoginTimeout
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(loginPoll):
		}
	}
}

func harvestLoggedIn(ctx context.Context, bc playwright.BrowserContext, page playwright.Page, recorder *headerRecorder) (*session.Session, error) {
	s, err := harvest(ctx, bc, page, recorder)
	if err != nil {
		return nil, err
	}

	if !s.LoggedIn() {
		return nil, errMissingCookies
	}

	return s, nil
}

func harvest(ctx context.Context, bc playwright.BrowserContext, page playwright.Page, recorder *headerRecorder) (*session.Session, error) {
	if err := ensureHome(page); err != nil {
		return nil, err
	}

	if err := waitForHeaders(ctx, recorder); err != nil {
		return nil, err
	}

	doc, fetch := recorder.snapshot()

	ua, err := pageUserAgent(page)
	if err != nil {
		return nil, err
	}
	if v := headerValue(doc, headerUserAgent); v != "" {
		ua = v
	}

	cookies, err := redditCookies(bc)
	if err != nil {
		return nil, err
	}

	return &session.Session{
		UserAgent:       ua,
		Cookies:         cookies,
		DocumentHeaders: doc,
		FetchHeaders:    fetch,
	}, nil
}

func ensureHome(page playwright.Page) error {
	if strings.HasPrefix(page.URL(), homeURL) && !strings.HasPrefix(page.URL(), loginURL) {
		return nil
	}

	_, err := page.Goto(homeURL, domLoaded)

	return err
}

func waitForHeaders(ctx context.Context, recorder *headerRecorder) error {
	deadline := time.Now().Add(harvestTimeout)

	for !recorder.complete() {
		if time.Now().After(deadline) {
			if doc, _ := recorder.snapshot(); doc == nil {
				return errNoNavigation
			}

			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(harvestPoll):
		}
	}

	return nil
}

func pageUserAgent(page playwright.Page) (string, error) {
	v, err := page.Evaluate("() => navigator.userAgent")
	if err != nil {
		return "", fmt.Errorf("read user agent: %w", err)
	}

	ua, _ := v.(string)

	return ua, nil
}

func redditCookies(bc playwright.BrowserContext) ([]session.Cookie, error) {
	all, err := bc.Cookies()
	if err != nil {
		return nil, err
	}

	out := make([]session.Cookie, 0, len(all))
	for _, c := range all {
		if !strings.HasSuffix(c.Domain, redditDomain) {
			continue
		}

		out = append(out, session.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HttpOnly: c.HttpOnly,
			Secure:   c.Secure,
		})
	}

	return out, nil
}
