# subgolem

Generate English SRT subtitles from non-English video using a local [whisper.cpp](https://github.com/ggml-org/whisper.cpp) model.

## Requirements

- Go 1.21+
- CMake 3.16+
- FFmpeg (must be on `$PATH`)
- GCC / clang
- (Optional) Vulkan SDK for GPU acceleration — auto-detected

## Setup

```sh
make setup   # downloads whisper.cpp source and builds the C library
```

This only needs to run once. The built library lives in `data/whisper-src/`.

## Build

```sh
make build
```

Output binary: `bin/subgolem`

Force CPU-only build (skips Vulkan detection):

```sh
make build-cpu
```

## Run

```sh
bin/subgolem -i video.mkv
```

The model is downloaded automatically on first run (to `data/models/`).

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | — | Input video or audio file (required) |
| `-o` | `<input>.srt` | Output SRT file |
| `--model` | `large-v3` | Whisper model: `tiny`, `base`, `small`, `medium`, `large-v3` |
| `--language` | `auto` | Source language code (e.g. `he`, `en`) or `auto` |
| `--translator` | `whisper` | Translation backend: `whisper` or `openai` |
| `--data-dir` | `data` | Directory for models and temp files |

### Examples

```sh
# Hebrew video → English SRT (default)
bin/subgolem -i movie.mkv

# Explicit language, custom output path
bin/subgolem -i movie.mkv -o subs/movie.srt --language he

# Use OpenAI / LM Studio for translation instead
bin/subgolem -i movie.mkv --translator openai
```

## Configuration

Create `config.yaml` in the working directory to persist settings:

```yaml
model: large-v3
language: auto
translator: whisper
data_dir: data

openai:
  base_url: http://localhost:1234/v1
  api_key: ""
  model: gpt-4o-mini
```

Environment variables override config values: `SUBGOLEM_MODEL`, `SUBGOLEM_LANGUAGE`, etc.

## GPU Acceleration

Vulkan is auto-detected at build time via `vulkaninfo` or `ldconfig`. No extra configuration needed — if Vulkan is available, it will be used.

## Tests

```sh
make test   # does not require make setup
```

## Clean

```sh
make clean      # removes bin/ and whisper build artifacts
make clean-all  # removes everything including downloaded models
```
