package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	homeURL  = "https://www.reddit.com/"
	loginURL = "https://www.reddit.com/login/"

	redditDomain = "reddit.com"
	wwwHost      = "www.reddit.com"

	cacheDir   = "reddit-mcp"
	profileDir = "profile"
	dirPerm    = 0o700

	viewportWidth  = 1280
	viewportHeight = 860

	closeWait = 2 * time.Second
	closePoll = 100 * time.Millisecond
)

type Browser struct {
	pw      *playwright.Playwright
	Context playwright.BrowserContext
}

func ProfileDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, cacheDir, profileDir)

	return dir, os.MkdirAll(dir, dirPerm)
}

func Launch(executable string, visible bool) (*Browser, error) {
	opts := &playwright.RunOptions{
		SkipInstallBrowsers: true,
		Stdout:              os.Stderr,
		Stderr:              os.Stderr,
	}
	if err := playwright.Install(opts); err != nil {
		return nil, fmt.Errorf("install playwright driver: %w", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}

	profile, err := ProfileDir()
	if err != nil {
		pw.Stop()
		return nil, err
	}

	clearStaleLock(profile)

	ctx, err := pw.Firefox.LaunchPersistentContext(profile, playwright.BrowserTypeLaunchPersistentContextOptions{
		ExecutablePath: playwright.String(executable),
		Headless:       playwright.Bool(!visible),
		Viewport:       &playwright.Size{Width: viewportWidth, Height: viewportHeight},
		Env:            fingerprintEnv(executable),
	})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("launch camoufox: %w", err)
	}

	return &Browser{pw: pw, Context: ctx}, nil
}

func (b *Browser) Close() {
	if b == nil {
		return
	}

	b.Context.Close()

	if br := b.Context.Browser(); br != nil {
		br.Close()
	}

	if profile, err := ProfileDir(); err == nil {
		waitExited(profile, closeWait)
	}

	b.pw.Stop()
}

func (b *Browser) Page() (playwright.Page, error) {
	if pages := b.Context.Pages(); len(pages) > 0 {
		return pages[0], nil
	}

	return b.Context.NewPage()
}

func RemoveProfile() error {
	profile, err := ProfileDir()
	if err != nil {
		return err
	}

	if pid, ok := lockOwner(profile); ok && alive(pid) {
		return fmt.Errorf("browser is still running (pid %d); close it first", pid)
	}

	return os.RemoveAll(profile)
}
