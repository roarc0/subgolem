# subgolem

Local transcription and English subtitle refinement.

Generate polished English SRT subtitles quickly and privately using local [whisper.cpp](https://github.com/ggml-org/whisper.cpp) models and LLM refinement pass.

## Key Features

- **Local-First**: Transcription and refinement occur entirely on your machine.
- **Vulkan Accelerated**: Auto-detects GPU support for both Whisper and LLMs.
- **Combined Pipeline**: Translates and polishes subtitles in a single context-aware pass.
- **HF Integration**: Supports dynamic model downloading from Hugging Face.

## Requirements

- Go 1.21+
- CMake 3.16+
- FFmpeg (must be on `$PATH`)
- (Optional) Vulkan SDK for GPU acceleration

## Setup

```sh
make setup   # downloads whisper.cpp source and builds the static library
make build   # produces bin/subgolem
```

---

## 5-Step Pipeline

1. **Extracting audio**: High-speed extraction with FFT normalisation and bandpass filtering.
2. **Downloading whisper model**: Efficiently manages local model storage and Hugging Face pulls.
3. **Transcribing**: Multi-threaded local inference via `whisper.cpp`.
4. **Writing subtitles**: Initial output of original language transcription.
5. **Translating and refining**: Single-pass LLM pass to translate and polish into natural English.

---

## Usage

```sh
bin/subgolem -i video.mkv
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | — | Input video or audio file (required) |
| `-o` | `<input>.srt` | Output SRT file |
| `--model` | `large-v3` | Model name or Hugging Face key (e.g. `ivrit-ai/whisper-large-v3-ggml`) |
| `--language` | `auto` | Source language code (e.g. `he`, `en`) or `auto` |
| `--data-dir` | `data` | Directory for models and temp files |

### Post-Processing
Subgolem can automatically clean up Whisper hallucinations, merge short segments, and fix timing overlaps. Enable these in `config.yaml`.

---

## Configuration (`config.yaml`)

```yaml
model: ivrit-ai/whisper-large-v3-ggml
language: auto
data_dir: data

# Transcriber settings
beam_size: 0
chunk_size: 300
prompt: "hebrew to english"

# LLM Refinement & Translation
llm_refine:
  enabled: true
  backend: "llamacpp"   # 'ollama' or 'llamacpp'
  
  # Custom prompt support with dynamic language detection
  prompt: "Translate from {{.SourceLang}} to natural, idiomatic English. Preserve meaning and fix grammar..."

# Custom HF Model Mapping
whisper_models:
  ivrit-ai/whisper-large-v3-ggml: "https://huggingface.co/ivrit-ai/whisper-large-v3-ggml/resolve/main/ggml-model.bin"
```

---

## LLM Refinement Setup

The refinement pass requires a local OpenAI-compatible API.

### Option A: llama.cpp (Recommended for Vulkan/GPU)
Run the included setup script:
```sh
scripts/setup-llamacpp.sh
make llm-server
```

### Option B: Ollama
Ensure Ollama is running, and set `backend: "ollama"` in `config.yaml`.

---

## Maintenance

```sh
make clean      # removes binaries and builds
make clean-all  # removes everything including downloaded models
make test       # runs core logic tests
```
