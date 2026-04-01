package segment

import (
	"regexp"
	"strings"
)

// artifactRE matches common whisper hallucinations: [Music], (inaudible), ♪, etc.
var artifactRE = regexp.MustCompile(`(?i)\[[^\]]*\]|\([^)]*\)|[♪♫]`)

// repeatedRE catches simple repetitions: "Thank you. Thank you."
var repeatedRE = regexp.MustCompile(`(.{10,}?)\s+\1`)

// Clean strips whisper hallucination artifacts and drops segments that are
// empty after cleaning.
func Clean(segs []Segment) []Segment {
	out := make([]Segment, 0, len(segs))
	for _, s := range segs {
		text := artifactRE.ReplaceAllString(s.Text, "")
		text = repeatedRE.ReplaceAllString(text, "$1")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		s.Text = text
		out = append(out, s)
	}
	return out
}
