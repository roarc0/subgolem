package segment

import (
	"regexp"
	"strings"
)

// artifactRE matches common whisper hallucinations: [Music], (inaudible), ♪, etc.
var artifactRE = regexp.MustCompile(`(?i)\[[^\]]*\]|\([^)]*\)|[♪♫]`)

// Clean strips whisper hallucination artifacts, collapses repeated phrases,
// and drops segments that are empty after cleaning.
func Clean(segs []Segment) []Segment {
	out := make([]Segment, 0, len(segs))
	for _, s := range segs {
		text := artifactRE.ReplaceAllString(s.Text, "")
		text = collapseRepeats(strings.TrimSpace(text))
		if text == "" {
			continue
		}
		s.Text = text
		out = append(out, s)
	}
	return out
}

// collapseRepeats removes consecutive duplicate phrases separated by whitespace.
// e.g. "Thank you. Thank you. Thank you." → "Thank you."
func collapseRepeats(text string) string {
	if len(text) < 20 {
		return text
	}
	// try every possible phrase length from half the string down to 10 chars
	for phraseLen := len(text) / 2; phraseLen >= 10; phraseLen-- {
		phrase := text[:phraseLen]
		rest := strings.TrimLeft(text[phraseLen:], " \t")
		if strings.HasPrefix(rest, phrase) {
			// repeated — keep one copy and recurse for triple+ repeats
			return collapseRepeats(phrase)
		}
	}
	return text
}
