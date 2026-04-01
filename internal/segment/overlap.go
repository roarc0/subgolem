package segment

import "time"

const minSegmentGap = time.Millisecond

// FixOverlaps ensures no segment's End exceeds the next segment's Start.
// When an overlap is detected the earlier segment is trimmed.
func FixOverlaps(segs []Segment) []Segment {
	for i := 0; i < len(segs)-1; i++ {
		limit := segs[i+1].Start - minSegmentGap
		if segs[i].End > limit {
			if limit > segs[i].Start {
				segs[i].End = limit
			} else {
				segs[i].End = segs[i].Start
			}
		}
	}
	return segs
}
