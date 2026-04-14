# subgolem Agent Guidelines

## Context
`subgolem` is a Go CLI application that generates English SRT subtitles from non-English videos using a local `whisper.cpp` model. It relies on CGO to interface with the C/C++ library and extracts audio via `ffmpeg`.

## Tech Stack
- **Language**: Go 1.21+
- **Core Dependencies**: `whisper.cpp` (built locally), FFmpeg (expected on `$PATH`), CMake 3.16+ (for building whisper.cpp).
- **Optional Check**: Vulkan SDK for GPU acceleration.

## Codebase Structure
- `cmd/`: Application entrypoint.
- `internal/`: Core business logic (audio extraction, transcription, translation backends).
- `data/`: Local storage for downloaded whisper models, temporary files, and downloaded whisper.cpp C source code.
- `bin/`: Compiled output binaries.
- `Makefile`: Standard orchestrator for all project commands.

## Workflows & Commands
- `make setup`: Downloads whisper.cpp source and builds the static C library (Run once before building).
- `make build`: Standard build (produces `bin/subgolem` with auto-detected Vulkan support).
- `make build-cpu`: Forces a CPU-only build.
- `make test`: Runs unit tests.
- `make clean` / `make clean-all`: Cleans binary/C artifacts and downloaded models.

## Development Guidelines
1. **Always use the Makefile**: Do not use raw `go build` because the project depends on proper CGO linker flags and pre-built `whisper.cpp` objects. 
2. **CGO Awareness**: The core transcription engine is in C/C++. Modifications to audio processing might require understanding CGO boundaries and memory management.
3. **Configuration**: The app uses both CLI flags and `config.yaml`. New settings should support both.
4. **Data Directories**: Treat `data/` as the dumping ground for large/temporary files (models, audio rips). Do not commit its contents unless specified by `.gitignore`.

## Coding Standards & Best Practices
1. **Clean Code**: Prioritize code readability and maintainability. Avoid deeply nested logic, use descriptive naming conventions, and keep functions small and focused on a single responsibility.
2. **Separation of Concerns**: Keep business logic decoupled from CLI/infrastructure concerns. The `cmd/` directory should only handle configuration and flag parsing, delegating execution to the packages within `internal/` which house the core features like audio extraction and transcription logic.
3. **Robust Testing**: Verify core logic with comprehensive unit tests (`make test`). Use mocks or interface-based designs to isolate components that depend on external side-effects (e.g., FFMpeg shell execution, model loading, API calls).
4. **Error Handling**: Use idiomatic Go error handling. Wrap errors with meaningful context at the boundary layers using `fmt.Errorf("...: %w", err)` rather than returning bare errors.
5. **Component Design**: Prefer passing structured configuration objects and minimal interfaces over global states or large parameter lists to ensure components remain testable and composable.
