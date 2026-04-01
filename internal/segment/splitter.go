package segment

import (
	"strings"
	"time"
	"unicode"
)

// Split breaks any segment whose text exceeds maxChars into smaller ones.
// Splits prefer sentence endings (.?!) then word boundaries.
// Timing is divided proportionally by character count.
func Split(segs []Segment, maxChars int) []Segment {
	if maxChars <= 0 {
		return segs
	}
	out := make([]Segment, 0, len(segs))
	for _, s := range segs {
		out = append(out, splitOne(s, maxChars)...)
	}
	return out
}

func splitOne(s Segment, maxChars int) []Segment {
	if len(s.Text) <= maxChars {
		return []Segment{s}
	}
	at := findSplitPoint(s.Text, maxChars)
	if at <= 0 {
		return []Segment{s}
	}

	a := strings.TrimSpace(s.Text[:at])
	b := strings.TrimSpace(s.Text[at:])
	if a == "" || b == "" {
		return []Segment{s}
	}

	// divide duration proportionally by character count
	total := s.End - s.Start
	mid := s.Start + time.Duration(float64(total)*float64(len(a))/float64(len(a)+len(b)))

	return append(
		splitOne(Segment{Start: s.Start, End: mid, Text: a}, maxChars),
		splitOne(Segment{Start: mid, End: s.End, Text: b}, maxChars)...,
	)
}

// findSplitPoint returns the best index to split text at, searching backwards
// from maxChars. Prefers a sentence boundary (after .?!), falls back to any space.
func findSplitPoint(text string, maxChars int) int {
	limit := maxChars
	if limit >= len(text) {
		return -1
	}
	// prefer split after sentence-ending punctuation
	for i := limit; i > 0; i-- {
		if i < len(text) && unicode.IsSpace(rune(text[i])) {
			prev := rune(text[i-1])
			if prev == '.' || prev == '?' || prev == '!' {
				return i
			}
		}
	}
	// fall back to any word boundary
	for i := limit; i > 0; i-- {
		if unicode.IsSpace(rune(text[i])) {
			return i
		}
	}
	return -1
}
