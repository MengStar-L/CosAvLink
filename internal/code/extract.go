// Package code extracts a normalized JAV product code (e.g. "DSAM-002") from
// the messy titles and cover filenames found on cosplay.jav.pw.
//
// Strategy (layered, first hit wins):
//  1. Title with an explicit separator: "[DSAM-002] ...", "CME-003 ひな".
//     This is how retail/MGStage posts are formatted and is the most reliable.
//  2. Cover-image filename: ".../dsam002pl.jpg" -> "DSAM-002". Used when the
//     title has no separator-form code.
//
// Items with neither (most doujin/cosplay posts, e.g. "SexFriend 227「...」")
// return "" and are treated as "no JAV code — skip javdb".
package code

import (
	"path"
	"regexp"
	"strings"
)

// titleSep matches a label of 2–6 letters, a "-" or "_" separator, then 2–5
// digits — the canonical printed form of a JAV code. Requiring the separator
// avoids false positives on series names like "SexFriend 227".
var titleSep = regexp.MustCompile(`(?i)([A-Za-z]{2,6})[-_](\d{2,5})`)

// fileCode matches the leading "letters+digits" of a cover filename such as
// "dsam002pl" or "cme0003jp". Purely numeric names (doujin) won't match.
var fileCode = regexp.MustCompile(`(?i)^([A-Za-z]{2,6})(\d{2,5})`)

// Extract returns the normalized code for a video, or "" when none is found.
func Extract(title, coverURL string) string {
	if m := titleSep.FindStringSubmatch(title); m != nil {
		return normalize(m[1], m[2], false)
	}
	if base := basename(coverURL); base != "" {
		if m := fileCode.FindStringSubmatch(base); m != nil {
			return normalize(m[1], m[2], true)
		}
	}
	return ""
}

// normalize formats letters+digits as "UPPER-###". For codes taken from a
// filename (fromFile), leading zeros are collapsed and re-padded to the common
// 3-digit width (e.g. "0003" -> "003"); codes printed in a title are kept
// verbatim since that form is authoritative.
func normalize(letters, digits string, fromFile bool) string {
	letters = strings.ToUpper(letters)
	if fromFile {
		trimmed := strings.TrimLeft(digits, "0")
		if trimmed == "" {
			trimmed = "0"
		}
		for len(trimmed) < 3 {
			trimmed = "0" + trimmed
		}
		digits = trimmed
	}
	return letters + "-" + digits
}

func basename(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	// Drop any query/fragment, then take the last path segment.
	if i := strings.IndexAny(rawURL, "?#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	return path.Base(rawURL)
}
