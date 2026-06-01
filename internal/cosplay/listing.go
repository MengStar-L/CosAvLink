// Package cosplay scrapes the latest-videos listing from cosplay.jav.pw.
//
// The site is a public WordPress install that responds to a plain HTTP GET
// (no Cloudflare), so a normal net/http client plus goquery is enough. Each
// listing item looks like:
//
//	<div class="post post-39157 ..." id="post-39157">
//	  <h2 class="title"><a href="PERMALINK" title="Permalink to ...">TITLE</a></h2>
//	  ...<p><a href="IMG"><img src=".../wp-content/uploads/.../x.jpg"></a>...
//	</div>
package cosplay

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"cosavlink/internal/cache"
	"cosavlink/internal/code"
	"cosavlink/internal/model"
)

const (
	homeURL   = "https://cosplay.jav.pw/"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
	cacheKey  = "home"
	cacheTTL  = 10 * time.Minute
)

// Client fetches and parses the cosplay.jav.pw listing, caching the result.
type Client struct {
	http  *http.Client
	cache *cache.TTL[string, []model.Video]
}

// New returns a Client with a sensible HTTP timeout.
func New() *Client {
	return &Client{
		http:  &http.Client{Timeout: 20 * time.Second},
		cache: cache.New[string, []model.Video](),
	}
}

// Latest returns the videos on the first listing page, using a short-lived
// cache so repeated page loads don't refetch the source on every request.
func (c *Client) Latest(ctx context.Context) ([]model.Video, error) {
	if v, ok := c.cache.Get(cacheKey); ok {
		return v, nil
	}
	videos, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cacheKey, videos, cacheTTL)
	return videos, nil
}

func (c *Client) fetch(ctx context.Context) ([]model.Video, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, homeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,ja;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch cosplay home: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cosplay home returned HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse cosplay home: %w", err)
	}

	var videos []model.Video
	doc.Find(`div[id^="post-"]`).Each(func(_ int, s *goquery.Selection) {
		link := s.Find("h2.title a").First()
		title := strings.TrimSpace(link.Text())
		if title == "" {
			return
		}
		detail, _ := link.Attr("href")

		// Prefer the uploaded cover image; fall back to the first image.
		cover, ok := s.Find(`img[src*="wp-content/uploads"]`).First().Attr("src")
		if !ok {
			cover, _ = s.Find("img").First().Attr("src")
		}

		videos = append(videos, model.Video{
			Title:     title,
			Code:      code.Extract(title, cover),
			Cover:     strings.TrimSpace(cover),
			DetailURL: strings.TrimSpace(detail),
		})
	})

	if len(videos) == 0 {
		return nil, fmt.Errorf("no videos parsed from cosplay home (page layout may have changed)")
	}
	return videos, nil
}
