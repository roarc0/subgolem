package subtitle

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/roarc0/subgolem/internal/segment"
)

// WriteSRT writes segments to path in SRT format (CRLF line endings per spec).
func WriteSRT(path string, segments []segment.Segment) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create SRT: %w", err)
	}
	defer f.Close()

	for i, seg := range segments {
		_, err := fmt.Fprintf(f, "%d\r\n%s --> %s\r\n%s\r\n\r\n",
			i+1,
			formatSRTTime(seg.Start),
			formatSRTTime(seg.End),
			strings.TrimSpace(seg.Text),
		)
		if err != nil {
			return fmt.Errorf("write segment %d: %w", i+1, err)
		}
	}
	return nil
}

func formatSRTTime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
