package segment

import "time"

// Merge joins consecutive segments where the inter-segment gap is ≤ maxGap
// and the combined text fits within maxChars. This produces longer, more
// natural-reading subtitle lines from whisper's fine-grained output.
func Merge(segs []Segment, maxGap time.Duration, maxChars int) []Segment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]Segment, 0, len(segs))
	out = append(out, segs[0])
	for _, s := range segs[1:] {
		last := &out[len(out)-1]
		gap := s.Start - last.End
		combined := last.Text + " " + s.Text
		if gap >= 0 && gap <= maxGap && len(combined) <= maxChars {
			last.End = s.End
			last.Text = combined
		} else {
			out = append(out, s)
		}
	}
	return out
}
