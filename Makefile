WHISPER_VERSION := 1.8.4
WHISPER_URL     := https://github.com/ggml-org/whisper.cpp/archive/refs/tags/v$(WHISPER_VERSION).tar.gz
WHISPER_SRC     := data/whisper-src/whisper.cpp-$(WHISPER_VERSION)
WHISPER_BUILD   := $(WHISPER_SRC)/build
WHISPER_LIB     := $(WHISPER_BUILD)/src/libwhisper.a

# Vulkan auto-detection: check for vulkaninfo or the Vulkan loader library
VULKAN_OK := $(shell (command -v vulkaninfo >/dev/null 2>&1 && vulkaninfo >/dev/null 2>&1) && echo 1 || \
             (ldconfig -p 2>/dev/null | grep -q libvulkan && echo 1) || echo 0)

ifeq ($(VULKAN_OK),1)
  CMAKE_FLAGS    := -DGGML_VULKAN=ON
  VULKAN_LDFLAGS := -L$(PWD)/$(WHISPER_BUILD)/ggml/src/ggml-vulkan -lggml-vulkan -lvulkan
  $(info [subgolem] Vulkan detected — GPU acceleration enabled)
else
  CMAKE_FLAGS    :=
  VULKAN_LDFLAGS :=
  $(info [subgolem] Vulkan not detected — CPU-only build)
endif

CGO_CFLAGS_VAL  := -I$(PWD)/$(WHISPER_SRC)/include -I$(PWD)/$(WHISPER_SRC)/ggml/include
CGO_LDFLAGS_VAL := -L$(PWD)/$(WHISPER_BUILD)/src \
                   -L$(PWD)/$(WHISPER_BUILD)/ggml/src \
                   -L$(PWD)/$(WHISPER_BUILD)/ggml/src/ggml-vulkan \
                   -lwhisper -lggml -lggml-base -lggml-cpu \
                   $(VULKAN_LDFLAGS) \
                   -lstdc++ -lm -fopenmp

.PHONY: all setup setup-ollama setup-llamacpp llm-server build build-cpu run test clean clean-all help

all: build

## setup    — download whisper.cpp source, build C library, create go.work
setup: $(WHISPER_LIB) go.work

$(WHISPER_SRC):
	mkdir -p data/whisper-src
	curl -fL $(WHISPER_URL) -o data/whisper-src/whisper.cpp-$(WHISPER_VERSION).tar.gz
	tar xzf data/whisper-src/whisper.cpp-$(WHISPER_VERSION).tar.gz -C data/whisper-src/
	rm data/whisper-src/whisper.cpp-$(WHISPER_VERSION).tar.gz

$(WHISPER_LIB): $(WHISPER_SRC)
	cmake -S $(WHISPER_SRC) -B $(WHISPER_BUILD) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		$(CMAKE_FLAGS)
	cmake --build $(WHISPER_BUILD) -j$$(nproc)

go.work: $(WHISPER_SRC)
	go work init . $(WHISPER_SRC)/bindings/go

## build    — compile subgolem binary to bin/subgolem (requires setup)
build: setup
	mkdir -p bin
	CGO_CFLAGS="$(CGO_CFLAGS_VAL)" CGO_LDFLAGS="$(CGO_LDFLAGS_VAL)" \
	go build -o bin/subgolem ./cmd/subgolem/
	@echo "[subgolem] Binary: bin/subgolem"

## build-cpu — force a CPU-only build (no Vulkan), rebuilds whisper library
build-cpu: $(WHISPER_SRC) go.work
	rm -rf $(WHISPER_BUILD)
	cmake -S $(WHISPER_SRC) -B $(WHISPER_BUILD) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF
	cmake --build $(WHISPER_BUILD) -j$$(nproc)
	mkdir -p bin
	CGO_CFLAGS="$(CGO_CFLAGS_VAL)" \
	CGO_LDFLAGS="-L$(PWD)/$(WHISPER_BUILD)/src -L$(PWD)/$(WHISPER_BUILD)/ggml/src -L$(PWD)/$(WHISPER_BUILD)/ggml/src/ggml-vulkan -lwhisper -lggml -lggml-base -lggml-cpu -lstdc++ -lm -fopenmp" \
	go build -o bin/subgolem ./cmd/subgolem/

## run      — go run subgolem (requires setup); pass args via ARGS="-i file.mp4"
run: setup
	CGO_CFLAGS="$(CGO_CFLAGS_VAL)" CGO_LDFLAGS="$(CGO_LDFLAGS_VAL)" \
	go run ./cmd/subgolem/ $(ARGS)

## test     — run unit tests (does NOT require setup)
test:
	go test ./internal/segment/... ./internal/subtitle/... ./internal/translate/... \
	        ./internal/models/... ./internal/audio/... -v

## clean    — remove bin/ and whisper build artifacts (keeps data/models/)
clean:
	rm -rf bin/ $(WHISPER_BUILD)

## clean-all — remove everything including downloaded models and whisper source
clean-all:
	rm -rf bin/ data/models/ data/whisper-src/ data/llama-bin/ go.work go.work.sum

## setup-ollama  — install Ollama and pull qwen2.5:7b (CPU inference)
setup-ollama:
	./scripts/setup-ollama.sh

## setup-llamacpp — download prebuilt llama.cpp Vulkan binary + Qwen2.5 GGUF model
setup-llamacpp:
	./scripts/setup-llamacpp.sh

## llm-server    — start llama.cpp server on :8080 (run after setup-llamacpp)
llm-server:
	LD_LIBRARY_PATH=$(PWD)/data/llama-bin$${LD_LIBRARY_PATH:+:$$LD_LIBRARY_PATH} \
	data/llama-bin/llama-server \
		-m data/models/Qwen2.5-7B-Instruct-Q4_K_M.gguf \
		-ngl 99 --port 8080 --ctx-size 4096

## help     — print available targets
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
