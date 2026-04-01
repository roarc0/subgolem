package subtitle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roarc0/subgolem/internal/segment"
	"github.com/roarc0/subgolem/internal/subtitle"
)

func TestWriteSRT(t *testing.T) {
	segs := []segment.Segment{
		{Start: 0, End: 3500 * time.Millisecond, Text: "Hello world"},
		{Start: 3500 * time.Millisecond, End: 7200 * time.Millisecond, Text: "Second line"},
	}

	out := filepath.Join(t.TempDir(), "test.srt")
	if err := subtitle.WriteSRT(out, segs); err != nil {
		t.Fatalf("WriteSRT: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	got := string(data)
	want := "1\r\n00:00:00,000 --> 00:00:03,500\r\nHello world\r\n\r\n" +
		"2\r\n00:00:03,500 --> 00:00:07,200\r\nSecond line\r\n\r\n"
	if got != want {
		t.Errorf("SRT output mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteSRT_Empty(t *testing.T) {
	out := filepath.Join(t.TempDir(), "empty.srt")
	if err := subtitle.WriteSRT(out, nil); err != nil {
		t.Fatalf("WriteSRT(nil): %v", err)
	}
	data, _ := os.ReadFile(out)
	if len(strings.TrimSpace(string(data))) != 0 {
		t.Error("expected empty file for nil segments")
	}
}
