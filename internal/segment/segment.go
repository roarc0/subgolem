package segment

import "time"

// Segment is a timed piece of transcribed and translated text.
type Segment struct {
	Start time.Duration
	End   time.Duration
	Text  string
}
