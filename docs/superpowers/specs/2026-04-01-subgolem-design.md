# subgolem — Design Spec

**Date:** 2026-04-01  
**Status:** Approved

## Overview

CLI tool that takes a video (or audio) file as input, transcribes the speech using a local whisper.cpp model, translates the result to English, and writes an SRT subtitle file. First use-case: Hebrew → English. Models are downloaded autonomously on first run.

---

## Architecture & Data Flow

```
input.mp4
    │
    ▼
[audio.Extractor]       FFmpeg: extract 16kHz mono WAV → .data/tmp/<hash>.wav
    │
    ▼
[models.Manager]        Check .data/models/ggml-<size>.bin
                        Download from Hugging Face CDN if missing (progress bar)
    │
    ▼
[transcribe.Transcriber] whisper.cpp CGo — runs "translate" task natively
                         Returns []subtitle.Segment{Start, End, Text}
    │
    ▼
[translate.Translator]  Interface — default: whisper passthrough (no-op)
                        Optional: OpenAI-compatible backend (LM Studio / OpenAI)
    │
    ▼
[subtitle.Writer]       Format segments → SRT, write output file
```

---

## Project Structure

```
subgolem/
├── cmd/subgolem/
│   └── main.go               # entrypoint, cobra root cmd, viper wiring
├── internal/
│   ├── audio/
│   │   └── extractor.go      # FFmpeg exec.Command wrapper
│   ├── models/
│   │   └── manager.go        # download, verify checksum, progress bar
│   ├── transcribe/
│   │   └── whisper.go        # CGo whisper.cpp wrapper, returns []Segment
│   ├── translate/
│   │   ├── translator.go     # Translator interface + Segment type
│   │   ├── whisper.go        # no-op passthrough (default)
│   │   └── openai.go         # OpenAI-compatible HTTP backend
│   └── subtitle/
│       └── writer.go         # SRT formatter and file writer
├── third_party/              # empty dir, whisper source downloads to .data/
├── Makefile                  # setup, build, build-cpu targets
├── config.yaml               # default config (committed, safe defaults)
├── .gitignore                # .data/, build artifacts
└── go.mod
```

---

## Core Interface

```go
// internal/translate/translator.go

type Segment struct {
    Start time.Duration
    End   time.Duration
    Text  string
}

type Translator interface {
    Translate(ctx context.Context, segments []Segment, sourceLang string) ([]Segment, error)
}
```

Implementations:
- `WhisperTranslator` — passthrough, whisper.cpp already translated during transcription
- `OpenAITranslator` — sends transcript to `/v1/chat/completions`, compatible with LM Studio

---

## CLI & Config

### Flags (Cobra + Viper)

```
subgolem -i input.mp4 -o output.srt [flags]

  -i, --input        Input video/audio file (required)
  -o, --output       Output SRT file (default: <input>.srt)
      --model        Model size: tiny|base|small|medium|large-v3 (default: large-v3)
      --language     Source language hint, e.g. "he" (default: auto)
      --translator   Backend: whisper|openai (default: whisper)
      --gpu          Enable Vulkan GPU acceleration (default: true)
      --data-dir     Model/temp dir (default: .data)
      --config       Config file (default: config.yaml)
```

### config.yaml

```yaml
model: large-v3
language: auto
translator: whisper
gpu: true
data_dir: .data

openai:
  base_url: http://localhost:1234/v1   # LM Studio or OpenAI
  api_key: ""
  model: gpt-4o-mini
```

Viper merge priority: **config file → env vars (`SUBGOLEM_*`) → CLI flags** (highest).

---

## Build System

whisper.cpp source is **not** a git submodule. `make setup` downloads the tagged tarball from GitHub releases into `.data/whisper-src/`, builds `libwhisper.a` there (with Vulkan if `libvulkan-dev` is present, otherwise CPU-only), and sets CGO flags accordingly.

```makefile
make setup        # download whisper.cpp source, build libwhisper.a (auto-detects Vulkan)
make build        # go build with CGO_LDFLAGS pointing to .data/whisper-src/
make build-cpu    # force CPU-only build (skips Vulkan detection)
make clean        # remove build artifacts (keeps downloaded models)
```

Build tags:
- `GGML_VULKAN=1` passed to CMake if `vulkaninfo` or `libvulkan.so` is found
- `CGO_CFLAGS` / `CGO_LDFLAGS` set by `make`, not hardcoded in Go files

---

## Model Management

- Models stored at `.data/models/ggml-<size>.bin`
- Source: `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-<size>.bin`
- SHA256 checksums hardcoded per model size in `models/manager.go`
- Download shows progress bar (via `schollz/progressbar`)
- Temp WAV files stored at `.data/tmp/`, cleaned up after each run

---

## SRT Output

Standard SRT format:
```
1
00:00:00,000 --> 00:00:03,500
Translated English text here.

2
00:00:03,500 --> 00:00:07,200
Next segment.
```

---

## Error Handling

- Missing `ffmpeg` binary: clear error message with install hint
- Download failure: retry once, then fatal with URL
- Checksum mismatch: delete corrupt file, prompt re-download
- whisper.cpp build failure: surface cmake stderr, suggest `make build-cpu`
- Input file not found / unsupported format: immediate exit with message

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/ggerganov/whisper.cpp/bindings/go` | CGo whisper bindings |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config management |
| `github.com/schollz/progressbar/v3` | Download progress |
| `github.com/sashabaranov/go-openai` | OpenAI-compatible translator backend |

---

## Non-Goals (v1)

- No batch processing (one file at a time)
- No subtitle burning into video
- No WebVTT output (SRT only)
- No GPU ROCm support (Vulkan covers AMD without extra setup)
- No streaming / real-time transcription
