// Package wikilink handles extraction and indexing of [[wikilinks]].
package wikilink

import (
	"regexp"
	"strings"
)

// WikiLink represents a single wikilink found in a document.
type WikiLink struct {
	Target string // the link target (e.g., "Trust as Social Contract")
	Alias  string // display text if using [[target|alias]] syntax, empty otherwise
	// Position in the source text.
	Start int
	End   int
}

// wikiLinkRe matches [[target]] and [[target|alias]] patterns.
// It does not match nested brackets or empty links.
var wikiLinkRe = regexp.MustCompile(`\[\[([^\[\]|]+?)(?:\|([^\[\]]+?))?\]\]`)

// Extract finds all wikilinks in the given text.
func Extract(text string) []WikiLink {
	matches := wikiLinkRe.FindAllStringSubmatchIndex(text, -1)
	links := make([]WikiLink, 0, len(matches))

	for _, match := range matches {
		// match[0:2] = full match
		// match[2:4] = target group
		// match[4:6] = alias group (-1 if not present)
		target := strings.TrimSpace(text[match[2]:match[3]])
		if target == "" {
			continue
		}

		wl := WikiLink{
			Target: target,
			Start:  match[0],
			End:    match[1],
		}

		if match[4] != -1 {
			wl.Alias = strings.TrimSpace(text[match[4]:match[5]])
		}

		links = append(links, wl)
	}

	return links
}

// Targets returns just the target names from the given text, deduplicated.
func Targets(text string) []string {
	links := Extract(text)
	seen := make(map[string]bool, len(links))
	targets := make([]string, 0, len(links))

	for _, l := range links {
		normalized := NormalizeTarget(l.Target)
		if !seen[normalized] {
			seen[normalized] = true
			targets = append(targets, l.Target)
		}
	}

	return targets
}

// NormalizeTarget normalizes a wikilink target for matching purposes.
// Lowercases and trims whitespace.
func NormalizeTarget(target string) string {
	return strings.ToLower(strings.TrimSpace(target))
}
