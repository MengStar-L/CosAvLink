// Package model defines the shared data structures used across CosAvLink.
package model

// Video is a single item scraped from the cosplay.jav.pw listing page.
type Video struct {
	// Title is the full post title as shown on cosplay.jav.pw.
	Title string `json:"title"`
	// Code is the normalized JAV product code (e.g. "DSAM-002") extracted
	// from the title or cover filename. Empty when no standard code was found.
	Code string `json:"code"`
	// Cover is the absolute URL of the cover image.
	Cover string `json:"cover"`
	// DetailURL is the cosplay.jav.pw post permalink.
	DetailURL string `json:"detailUrl"`
}

// HasCode reports whether a JAV product code was extracted, i.e. whether it is
// worth querying javdb.com for magnet links.
func (v Video) HasCode() bool { return v.Code != "" }

// Magnet is a single magnet link scraped from a javdb.com detail page.
type Magnet struct {
	// Name is the magnet display name (from the dn= parameter or the row text).
	Name string `json:"name"`
	// Link is the full "magnet:?xt=..." URI.
	Link string `json:"link"`
	// Size is the human-readable size string as shown on javdb (e.g. "5.2GB").
	Size string `json:"size"`
	// Date is the upload/publish date string when available.
	Date string `json:"date"`
	// Tags are extra labels such as "高清"/"HD"/"字幕".
	Tags []string `json:"tags"`
}

// MagnetResult is the cached outcome of a javdb lookup for one code.
type MagnetResult struct {
	// Code is the normalized code that was looked up.
	Code string `json:"code"`
	// Magnets is the list found (may be empty).
	Magnets []Magnet `json:"magnets"`
	// DetailURL is the javdb.com video page the magnets came from, if any.
	DetailURL string `json:"detailUrl"`
	// Blocked is true when the lookup failed because javdb blocked us
	// (Cloudflare), as opposed to simply finding no magnets.
	Blocked bool `json:"blocked"`
	// Note carries a short human-readable status for the UI (e.g. why empty).
	Note string `json:"note"`
}
