// Package javdb looks up magnet links on javdb.com for a given product code.
//
// javdb sits behind Cloudflare, so every lookup goes through FlareSolverr
// (see package flaresolverr). Flow per code:
//
//	/search?q=CODE&f=all  ->  pick best "a.box" result  ->  /v/XXXX detail page
//	->  parse "#magnets-content .item" rows for magnet URIs.
//
// Results are cached (positive long, negative/blocked short) and concurrent
// lookups of the same code are de-duplicated with singleflight.
package javdb

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/singleflight"

	"cosavlink/internal/cache"
	"cosavlink/internal/flaresolverr"
	"cosavlink/internal/model"
)

const (
	base          = "https://javdb.com"
	lookupTimeout = 90 * time.Second

	positiveTTL = 12 * time.Hour
	negativeTTL = 1 * time.Hour
	blockedTTL  = 2 * time.Minute
)

var sizeRe = regexp.MustCompile(`(?i)([\d.]+\s*[GMT]?B)`)

// Client performs javdb magnet lookups with caching + de-duplication.
type Client struct {
	fs    *flaresolverr.Client
	cache *cache.TTL[string, model.MagnetResult]
	sf    singleflight.Group
}

// New returns a javdb Client backed by the given FlareSolverr client.
func New(fs *flaresolverr.Client) *Client {
	return &Client{fs: fs, cache: cache.New[string, model.MagnetResult]()}
}

// Magnets returns the magnet result for a normalized code. It never returns a
// FlareSolverr/Cloudflare error to the caller — a block is reported via the
// result's Blocked/Note fields so the UI can render it gracefully.
func (c *Client) Magnets(ctx context.Context, rawCode string) (model.MagnetResult, error) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return model.MagnetResult{Note: "无番号，无法查询"}, nil
	}
	if v, ok := c.cache.Get(code); ok {
		return v, nil
	}

	v, err, _ := c.sf.Do(code, func() (any, error) {
		// Detach from the request context so one client disconnecting doesn't
		// abort a lookup shared by others.
		ctx2, cancel := context.WithTimeout(context.Background(), lookupTimeout)
		defer cancel()

		res := c.lookup(ctx2, code)

		ttl := positiveTTL
		switch {
		case res.Blocked:
			ttl = blockedTTL
		case len(res.Magnets) == 0:
			ttl = negativeTTL
		}
		c.cache.Set(code, res, ttl)
		return res, nil
	})
	if err != nil {
		return model.MagnetResult{Code: code}, err
	}
	return v.(model.MagnetResult), nil
}

// lookup performs the actual two-step fetch and parse through FlareSolverr.
// It always returns a populated result; transport/Cloudflare problems set
// Blocked + Note.
func (c *Client) lookup(ctx context.Context, code string) model.MagnetResult {
	res := model.MagnetResult{Code: code}

	// --- step 1: search ---
	searchURL := base + "/search?q=" + url.QueryEscape(code) + "&f=all"
	result, err := c.fs.Get(ctx, searchURL)
	if err != nil {
		res.Note = fmt.Sprintf("查询出错：%v", err)
		return res
	}

	title := extractTitle(result.HTML)
	if flaresolverr.LooksBlocked(title, result.HTML) {
		res.Blocked = true
		res.Note = "被 Cloudflare 拦截。请确认 FlareSolverr 已启动并正常运行"
		return res
	}

	if !strings.Contains(result.HTML, "movie-list") {
		res.Note = "javdb 未找到该番号"
		return res
	}

	detailURL := pickResult(result.HTML, code)
	if detailURL == "" {
		res.Note = "javdb 未找到匹配结果"
		return res
	}
	res.DetailURL = detailURL

	// --- step 2: detail page magnets ---
	result2, err := c.fs.Get(ctx, detailURL)
	if err != nil {
		res.Note = fmt.Sprintf("查询详情页出错：%v", err)
		return res
	}

	title2 := extractTitle(result2.HTML)
	if flaresolverr.LooksBlocked(title2, result2.HTML) {
		res.Blocked = true
		res.Note = "被 Cloudflare 拦截。请确认 FlareSolverr 已启动并正常运行"
		return res
	}

	if !strings.Contains(result2.HTML, "magnets-content") {
		res.Note = "未找到磁力（可能暂无资源，或需要登录查看）"
		return res
	}

	res.Magnets = parseMagnets(result2.HTML)
	if len(res.Magnets) == 0 {
		res.Note = "该番号在 javdb 上暂无磁力（部分资源需登录）"
	}
	return res
}

// extractTitle extracts the <title> text from raw HTML.
func extractTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title>")
	if start < 0 {
		return ""
	}
	start += len("<title>")
	end := strings.Index(lower[start:], "</title>")
	if end < 0 {
		return ""
	}
	return html[start : start+end]
}

// pickResult chooses the best search result href. It prefers a result whose
// title's leading token equals the searched code, else the first result.
func pickResult(html, code string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	var first, exact string
	doc.Find("div.movie-list .item a.box").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return true
		}
		if first == "" {
			first = href
		}
		title := strings.TrimSpace(a.Find(".video-title").First().Text())
		if fields := strings.Fields(title); len(fields) > 0 && strings.EqualFold(fields[0], code) {
			exact = href
			return false
		}
		return true
	})
	href := exact
	if href == "" {
		href = first
	}
	return absURL(href)
}

func parseMagnets(html string) []model.Magnet {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}
	var out []model.Magnet
	doc.Find("#magnets-content .item").Each(func(_ int, s *goquery.Selection) {
		link := magnetLink(s)
		if link == "" {
			return
		}
		m := model.Magnet{
			Link: link,
			Name: magnetName(s, link),
			Size: magnetSize(s),
			Date: magnetDate(s),
		}
		s.Find(".tags .tag, .tag").Each(func(_ int, t *goquery.Selection) {
			if tag := strings.TrimSpace(t.Text()); tag != "" {
				m.Tags = append(m.Tags, tag)
			}
		})
		out = append(out, m)
	})
	return out
}

func magnetLink(s *goquery.Selection) string {
	var found string
	s.Find("[data-clipboard-text]").EachWithBreak(func(_ int, e *goquery.Selection) bool {
		if v, ok := e.Attr("data-clipboard-text"); ok && strings.HasPrefix(v, "magnet:") {
			found = v
			return false
		}
		return true
	})
	if found != "" {
		return found
	}
	if v, ok := s.Find(`a[href^="magnet:"]`).First().Attr("href"); ok {
		return v
	}
	return ""
}

func magnetName(s *goquery.Selection, link string) string {
	for _, sel := range []string{".name", "a.name", ".title"} {
		if n := strings.TrimSpace(s.Find(sel).First().Text()); n != "" {
			return n
		}
	}
	if dn := dnParam(link); dn != "" {
		return dn
	}
	return "magnet"
}

func magnetSize(s *goquery.Selection) string {
	meta := strings.TrimSpace(s.Find(".meta").First().Text())
	if m := sizeRe.FindString(meta); m != "" {
		return strings.TrimSpace(m)
	}
	return meta
}

func magnetDate(s *goquery.Selection) string {
	for _, sel := range []string{".date .time", ".time", ".date"} {
		if d := strings.TrimSpace(s.Find(sel).First().Text()); d != "" {
			return d
		}
	}
	return ""
}

// dnParam extracts and decodes the display-name (dn) parameter of a magnet URI.
func dnParam(magnet string) string {
	i := strings.Index(magnet, "?")
	if i < 0 {
		return ""
	}
	vals, err := url.ParseQuery(magnet[i+1:])
	if err != nil {
		return ""
	}
	return strings.TrimSpace(vals.Get("dn"))
}

func absURL(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http") {
		return href
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}
	return base + href
}