// Package browser manages a single, long-lived go-rod browser used to reach
// javdb.com through its Cloudflare protection.
//
// On Windows it auto-detects Edge or Chrome and runs in headless mode.
// A persistent profile dir stores cf_clearance cookies across runs.
// Each lookup borrows a fresh stealth page, bounded by a concurrency semaphore.
package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

// ErrBlocked indicates javdb served a Cloudflare block/challenge page.
var ErrBlocked = errors.New("blocked by Cloudflare")

// Options configures the browser Manager.
type Options struct {
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
			Headless(true).
			Leakless(false).
			Set("disable-blink-features", "AutomationControlled").
			Set("no-first-run").
			Set("no-default-browser-check")
		if m.opts.ProfileDir != "" {
			l = l.UserDataDir(m.opts.ProfileDir)
		}
		if bin, ok := launcher.LookPath(); ok {
			l = l.Bin(bin)
		}

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

// Close shuts down the browser if it was launched.
func (m *Manager) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	if m.closed || m.browser == nil {
		return nil
	}
	m.closed = true
	return m.browser.Close()
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
