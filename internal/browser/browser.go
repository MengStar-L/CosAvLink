// Package browser manages a single, long-lived go-rod browser used to reach
// javdb.com through its Cloudflare protection.
//
// Key anti-Cloudflare measures:
//   - Headful mode (real browser window) on Windows — far more reliable than
//     headless, which Cloudflare actively detects.
//   - Anti-automation Chrome flags + stealth JS patches.
//   - After navigation, polls up to 60s for the Cloudflare challenge to
//     resolve automatically (Turnstile runs in the background).
//   - Persistent profile dir stores cf_clearance cookie across runs.
package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

// ErrBlocked indicates javdb served a Cloudflare block/challenge page
// that did not resolve within the timeout.
var ErrBlocked = errors.New("blocked by Cloudflare")

// Options configures the browser Manager.
type Options struct {
	// Headless runs Chrome without a visible window. Default is headful
	// (visible) which is far more reliable against Cloudflare.
	Headless bool
	// ProfileDir is a dedicated Chrome user-data dir for persisting cookies.
	ProfileDir string
	// MaxParallel bounds concurrent open pages (default 2).
	MaxParallel int
}

// Manager owns the shared browser instance, launched lazily on first use.
type Manager struct {
	opts     Options
	sem      chan struct{}
	initOnce sync.Once
	browser  *rod.Browser
	launcher *launcher.Launcher
	initErr  error
	closeMu  sync.Mutex
	closed   bool
}

// New returns a Manager. Chrome is not launched until the first WithPage call.
func New(opts Options) *Manager {
	if opts.MaxParallel < 1 {
		opts.MaxParallel = 2
	}
	if opts.ProfileDir == "" {
		opts.ProfileDir = ".cosavlink-browser"
	}
	return &Manager{opts: opts, sem: make(chan struct{}, opts.MaxParallel)}
}

// ensure launches and connects to Chrome exactly once.
func (m *Manager) ensure() error {
	m.initOnce.Do(func() {
		l := launcher.New().
			Headless(m.opts.Headless).
			Leakless(false).
			Set("disable-blink-features", "AutomationControlled").
			Set("no-first-run").
			Set("no-default-browser-check").
			Set("disable-infobars").
			Set("disable-extensions").
			Set("disable-component-update").
			Set("disable-background-networking")

		if m.opts.ProfileDir != "" {
			l = l.UserDataDir(m.opts.ProfileDir)
		}
		if bin, ok := launcher.LookPath(); ok {
			l = l.Bin(bin)
		}
		m.launcher = l

		controlURL, err := l.Launch()
		if err != nil {
			m.initErr = fmt.Errorf("launch chrome: %w", err)
			return
		}
		b := rod.New().ControlURL(controlURL)
		if err := b.Connect(); err != nil {
			m.initErr = fmt.Errorf("connect chrome: %w", err)
			return
		}
		m.browser = b
		m.applyCookies()
	})
	return m.initErr
}

// applyCookies seeds javdb's age-gate cookie.
func (m *Manager) applyCookies() {
	cookies := []*proto.NetworkCookieParam{{
		Name: "over18", Value: "1", Domain: ".javdb.com", Path: "/",
	}}
	_ = m.browser.SetCookies(cookies)
}

// WithPage borrows a stealth page (bounded by the concurrency semaphore), runs
// fn with it, and always closes it afterwards.
func (m *Manager) WithPage(ctx context.Context, fn func(*rod.Page) error) error {
	if err := m.ensure(); err != nil {
		return err
	}
	select {
	case m.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-m.sem }()

	page, err := stealth.Page(m.browser)
	if err != nil {
		return fmt.Errorf("open stealth page: %w", err)
	}
	defer func() { _ = page.Close() }()

	return fn(page.Context(ctx))
}

// NavigateAndWait navigates to a URL and waits for the Cloudflare challenge
// to resolve. It polls the page every 2 seconds for up to 60 seconds,
// checking if the content has become real (not a challenge interstitial).
// Returns the page for further use; caller must NOT close it.
func (m *Manager) NavigateAndWait(ctx context.Context, url string) (*rod.Page, error) {
	if err := m.ensure(); err != nil {
		return nil, err
	}
	select {
	case m.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// NOTE: caller is responsible for calling releaseSem and closing page.

	page, err := stealth.Page(m.browser)
	if err != nil {
		<-m.sem
		return nil, fmt.Errorf("open stealth page: %w", err)
	}
	page = page.Context(ctx)

	if err := page.Navigate(url); err != nil {
		_ = page.Close()
		<-m.sem
		return nil, err
	}
	_ = page.WaitLoad()

	// Poll for challenge resolution.
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		title := pageTitle(page)
		html := pageHTML(page)

		if !LooksBlocked(title, html) && looksReal(html) {
			// Challenge resolved — return the page.
			// The semaphore slot will be released when the page is used and closed.
			return page, nil
		}

		select {
		case <-ctx.Done():
			_ = page.Close()
			<-m.sem
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	// Timeout — still blocked.
	_ = page.Close()
	<-m.sem
	return nil, ErrBlocked
}

// ReleaseSem releases one concurrency semaphore slot. Call this after you're
// done with a page obtained from NavigateAndWait.
func (m *Manager) ReleaseSem() {
	<-m.sem
}

// Close shuts down the browser and kills all Chrome processes if it was launched.
func (m *Manager) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true

	// First try graceful browser close.
	if m.browser != nil {
		_ = m.browser.Close()
	}

	// Then kill the Chrome process tree via the launcher (kills child processes too).
	if m.launcher != nil {
		m.launcher.Kill()
	}
	return nil
}

// looksReal reports whether HTML appears to be a genuine javdb page rather
// than a challenge/block interstitial.
func looksReal(html string) bool {
	for _, marker := range []string{`class="movie-list`, `id="videos"`, "navbar", "/v/", "magnets-content"} {
		if strings.Contains(html, marker) {
			return true
		}
	}
	return false
}

func pageTitle(page *rod.Page) string {
	info, err := page.Info()
	if err != nil || info == nil {
		return ""
	}
	return info.Title
}

func pageHTML(page *rod.Page) string {
	h, err := page.HTML()
	if err != nil {
		return ""
	}
	return h
}

// LooksBlocked reports whether the given page title/HTML is a Cloudflare
// block or "checking your browser" interstitial.
func LooksBlocked(title, html string) bool {
	t := strings.ToLower(title)
	for _, marker := range []string{"just a moment", "attention required", "you have been blocked"} {
		if strings.Contains(t, marker) {
			return true
		}
	}
	h := strings.ToLower(html)
	for _, marker := range []string{
		"/cdn-cgi/styles/cf.errors",
		"cf-error-details",
		"checking your browser before accessing",
		"sorry, you have been blocked",
	} {
		if strings.Contains(h, marker) {
			return true
		}
	}
	return false
}
