// Package javdb looks up magnet links on javdb.com for a given product code
// or title.
//
// javdb sits behind Cloudflare, so every lookup goes through FlareSolverr
// (see package flaresolverr). Flow:
//
//	/search?q=QUERY&f=all  ->  pick best result  ->  /v/XXXX detail page
//	->  parse "#magnets-content .item" rows for magnet URIs.
//	->  if no magnets, parse "#short-comments" for user-posted magnet links.
//
// Results are cached (positive long, negative/blocked short) and concurrent
// lookups of the same query are de-duplicated with singleflight.
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

var (
	sizeRe      = regexp.MustCompile(`(?i)([\d.]+\s*[GMT]?B)`)
	magnetLinkRe = regexp.MustCompile(`magnet:\?xt=urn:btih:[a-zA-Z0-9]+[^\s<"]*`)
)

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

// Magnets returns the magnet result for a code or title. When code is
// non-empty it is used for the search; otherwise title is used. It never
// returns a FlareSolverr/Cloudflare error to the caller — a block is
// reported via the result's Blocked/Note fields.
func (c *Client) Magnets(ctx context.Context, code, title string) (model.MagnetResult, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	title = strings.TrimSpace(title)

	if code == "" && title == "" {
		return model.MagnetResult{Note: "无番号且无标题，无法查询"}, nil
	}

	// Cache key: prefer code, fall back to title.
	cacheKey := code
	if cacheKey == "" {
		cacheKey = "title:" + title
	}
	if v, ok := c.cache.Get(cacheKey); ok {
		return v, nil
	}

	v, err, _ := c.sf.Do(cacheKey, func() (any, error) {
		ctx2, cancel := context.WithTimeout(context.Background(), lookupTimeout)
		defer cancel()

		var res model.MagnetResult
		if code != "" {
			res = c.lookup(ctx2, code, true)
		} else {
			res = c.lookup(ctx2, title, false)
		}

		ttl := positiveTTL
		switch {
		case res.Blocked:
			ttl = blockedTTL
		case len(res.Magnets) == 0:
			ttl = negativeTTL
		}
		c.cache.Set(cacheKey, res, ttl)
		return res, nil
	})
	if err != nil {
		return model.MagnetResult{Code: code}, err
	}
	return v.(model.MagnetResult), nil
}

// lookup performs the actual two-step fetch and parse through FlareSolverr.
// When isCode is true, the query is treated as a product code for exact
// matching; otherwise it is a title for fuzzy matching.
func (c *Client) lookup(ctx context.Context, query string, isCode bool) model.MagnetResult {
	res := model.MagnetResult{Code: query, Query: query}

	// --- step 1: search ---
	searchURL := base + "/search?q=" + url.QueryEscape(query) + "&f=all"
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
		res.Note = "javdb 未找到匹配结果"
		return res
	}

	var detailURL string
	if isCode {
		detailURL = pickResultByCode(result.HTML, query)
	} else {
		detailURL = pickResultByTitle(result.HTML, query)
	}
	if detailURL == "" {
		res.Note = "javdb 未找到匹配结果"
		return res
	}
	res.DetailURL = detailURL

	// --- step 2: detail page ---
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

	// Try main magnets section first.
	if strings.Contains(result2.HTML, "magnets-content") {
		res.Magnets = parseMagnets(result2.HTML)
	}

	// Fallback: if no magnets found, try extracting from short comments.
	if len(res.Magnets) == 0 {
		commentMagnets := parseCommentMagnets(result2.HTML)
		if len(commentMagnets) > 0 {
			res.Magnets = commentMagnets
			res.Note = "磁力来自 javdb 短评（用户分享）"
		} else {
			res.Note = "未找到磁力（可能暂无资源，或需要登录查看）"
		}
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

// pickResultByCode chooses the best search result href when searching by code.
// It prefers a result whose title's leading token equals the searched code.
func pickResultByCode(html, code string) string {
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

// pickResultByTitle chooses the best search result when searching by title.
// It uses fuzzy matching: any result whose title contains significant words
// from the search query is preferred; otherwise the first result is used.
func pickResultByTitle(html, query string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}

	queryWords := extractSignificantWords(query)
	var first, best string
	bestScore := 0

	doc.Find("div.movie-list .item a.box").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return true
		}
		if first == "" {
			first = href
		}
		title := strings.TrimSpace(a.Find(".video-title").First().Text())
		if title == "" {
			return true
		}

		// Score: number of query words found in the result title.
		score := 0
		titleLower := strings.ToLower(title)
		for _, w := range queryWords {
			if strings.Contains(titleLower, w) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = href
		}
		return true
	})

	href := best
	if href == "" || bestScore == 0 {
		href = first
	}
	return absURL(href)
}

// extractSignificantWords splits text into lowercase words of 2+ chars,
// filtering out common stop words and very short tokens.
func extractSignificantWords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var out []string
	for _, w := range words {
		// Strip punctuation from edges.
		w = strings.Trim(w, "[](){}.,;:!?\"'`~@#$%^&*+=/\\|<>")
		if len(w) >= 2 {
			out = append(out, w)
		}
	}
	return out
}

// parseMagnets extracts magnet links from the main #magnets-content section.
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

// parseCommentMagnets extracts magnet links posted by users in the short
// comments section (#short-comments). These are plain-text magnet URIs
// embedded in comment bodies.
func parseCommentMagnets(html string) []model.Magnet {
	seen := make(map[string]bool)
	var out []model.Magnet

	// Use regex to find all magnet links in the entire HTML.
	// This catches magnets in comments that may not be in proper link elements.
	for _, match := range magnetLinkRe.FindAllString(html, -1) {
		if seen[match] {
			continue
		}
		seen[match] = true
		out = append(out, model.Magnet{
			Link:   match,
			Name:   dnParam(match),
			Source: "comment",
		})
	}
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
