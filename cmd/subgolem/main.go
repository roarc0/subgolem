package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	".mp3": true, ".wav": true, ".flac": true, ".ogg": true,
	".m4a": true, ".aac": true,
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:          "subgolem -i <input> [-i <input2>] [-o <output.srt>]",
		Short:        "Generate English SRT subtitles from non-English video",
		Long:         "subgolem transcribes and translates video audio to English SRT subtitles using a local whisper.cpp model.",
		RunE:         run,
		SilenceUsage: true,
		Args:         cobra.ArbitraryArgs,
	}

	cobra.OnInitialize(func() { initConfig(cfgFile) })

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config.yaml)")
	cmd.Flags().StringArrayP("input", "i", nil, "input video/audio file or directory (repeatable)")
	cmd.Flags().StringP("output", "o", "", "output SRT file (only valid for single input)")
	cmd.Flags().String("model", "large-v3", "whisper model: tiny|base|small|medium|large-v3")
	cmd.Flags().String("language", "auto", "source language code (e.g. 'he') or 'auto'")
	cmd.Flags().String("translator", "whisper", "translation backend: whisper|openai")
	cmd.Flags().Bool("gpu", true, "enable Vulkan GPU acceleration")
	cmd.Flags().String("data-dir", "data", "directory for models and temp files")
	cmd.Flags().Bool("audio-filter", true, "apply loudness normalisation and speech bandpass filter (FFmpeg)")
	cmd.Flags().Int("beam-size", 0, "whisper beam search width (0 = use whisper internal default)")
	cmd.Flags().Int("chunk-size", 300, "size of transcription chunks in seconds (0 = disabled)")
	cmd.Flags().Bool("vad", false, "enable voice activity detection — requires --vad-model path")
	cmd.Flags().String("prompt", "", "initial prompt to guide whisper (e.g. domain vocabulary)")
	cmd.Flags().Bool("clean", true, "strip whisper hallucination artifacts ([Music], ♪, etc.)")
	cmd.Flags().Duration("merge-gap", 1*time.Second, "merge segments closer than this gap (0 = disabled)")
	cmd.Flags().Int("merge-chars", 80, "max characters per merged segment")
	cmd.Flags().Int("split-chars", 80, "split segments longer than this (0 = disabled)")
	cmd.Flags().Bool("fix-overlaps", true, "trim segments whose end time overlaps the next segment")

	viper.BindPFlag("model", cmd.Flags().Lookup("model"))
	viper.BindPFlag("language", cmd.Flags().Lookup("language"))
	viper.BindPFlag("translator", cmd.Flags().Lookup("translator"))
	viper.BindPFlag("gpu", cmd.Flags().Lookup("gpu"))
	viper.BindPFlag("data_dir", cmd.Flags().Lookup("data-dir"))
	viper.BindPFlag("audio_filter", cmd.Flags().Lookup("audio-filter"))
	viper.BindPFlag("beam_size", cmd.Flags().Lookup("beam-size"))
	viper.BindPFlag("chunk_size", cmd.Flags().Lookup("chunk-size"))
	viper.BindPFlag("vad", cmd.Flags().Lookup("vad"))
	viper.BindPFlag("prompt", cmd.Flags().Lookup("prompt"))
	viper.BindPFlag("clean", cmd.Flags().Lookup("clean"))
	viper.BindPFlag("merge_gap", cmd.Flags().Lookup("merge-gap"))
	viper.BindPFlag("merge_chars", cmd.Flags().Lookup("merge-chars"))
	viper.BindPFlag("split_chars", cmd.Flags().Lookup("split-chars"))
	viper.BindPFlag("fix_overlaps", cmd.Flags().Lookup("fix-overlaps"))

	return cmd
}

// muteCLibOutput redirects stdout and stderr fds to /dev/null for the duration
// of the TUI so that C library diagnostics (whisper, ggml) don't corrupt the display.
// bubbletea owns its own terminal handle (via /dev/tty) so the TUI is unaffected.
func muteCLibOutput() func() {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return func() {}
	}
	null := int(devNull.Fd())

	savedOut, errOut := syscall.Dup(1)
	savedErr, errErr := syscall.Dup(2)

	if errOut == nil {
		syscall.Dup2(null, 1) //nolint:errcheck
	}
	if errErr == nil {
		syscall.Dup2(null, 2) //nolint:errcheck
	}
	devNull.Close()

	return func() {
		if errOut == nil {
			syscall.Dup2(savedOut, 1) //nolint:errcheck
			syscall.Close(savedOut)
		}
		if errErr == nil {
			syscall.Dup2(savedErr, 2) //nolint:errcheck
			syscall.Close(savedErr)
		}
	}
}

func initConfig(cfgFile string) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}
	viper.SetEnvPrefix("SUBGOLEM")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	_ = viper.ReadInConfig()
}

func run(cmd *cobra.Command, args []string) error {
	// Collect inputs from -i flags and positional args.
	flagInputs, _ := cmd.Flags().GetStringArray("input")
	inputs := append(flagInputs, args...)

	if len(inputs) == 0 {
		return fmt.Errorf("input required — use -i <file|dir> or pass paths as arguments")
	}

	// Expand directories and validate files.
	var files []string
	for _, p := range inputs {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("input not found: %s", p)
		}
		if info.IsDir() {
			expanded, err := expandDir(p)
			if err != nil {
				return err
			}
			if len(expanded) == 0 {
				return fmt.Errorf("no supported video/audio files found in %s", p)
			}
			files = append(files, expanded...)
		} else {
			files = append(files, p)
		}
	}

	outputPath, _ := cmd.Flags().GetString("output")
	if outputPath != "" && len(files) > 1 {
		return fmt.Errorf("-o is only valid for a single input file")
	}

	translatorID := viper.GetString("translator")
	if translatorID != "whisper" && translatorID != "openai" {
		return fmt.Errorf("unknown translator %q — valid: whisper, openai", translatorID)
	}

	for i, inputPath := range files {
		out := outputPath
		if out == "" {
			ext := filepath.Ext(inputPath)
			out = strings.TrimSuffix(inputPath, ext) + ".srt"
		}

		cfg := PipelineConfig{
			InputPath:     inputPath,
			OutputPath:    out,
			ModelName:     viper.GetString("model"),
			Lang:          viper.GetString("language"),
			TranslatorID:  translatorID,
			DataDir:       viper.GetString("data_dir"),
			OpenAIBaseURL: viper.GetString("openai.base_url"),
			OpenAIAPIKey:  viper.GetString("openai.api_key"),
			OpenAIModel:   viper.GetString("openai.model"),
			AudioFilter:   viper.GetBool("audio_filter"),
			BeamSize:      viper.GetInt("beam_size"),
			ChunkSize:     viper.GetInt("chunk_size"),
			VAD:           viper.GetBool("vad"),
			Prompt:        viper.GetString("prompt"),
			Clean:         viper.GetBool("clean"),
			MergeGap:      viper.GetDuration("merge_gap"),
			MergeChars:    viper.GetInt("merge_chars"),
			SplitChars:    viper.GetInt("split_chars"),
			FixOverlaps:   viper.GetBool("fix_overlaps"),
			FileIndex:     i + 1,
			FileCount:     len(files),
			RefinerEnabled: viper.GetBool("llm_refine.enabled"),
			RefinerBaseURL: viper.GetString("llm_refine.base_url"),
			RefinerAPIKey:  viper.GetString("llm_refine.api_key"),
			RefinerModel:   viper.GetString("llm_refine.model"),
			RefinerChunk:   viper.GetInt("llm_refine.chunk_size"),
			RefinerPrompt:  viper.GetString("llm_refine.prompt"),
		}

		tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("open tty: %w", err)
		}
		// Package-level styles already hold a pointer to the existing default renderer,
		// so replacing it with SetDefaultRenderer has no effect on them.
		// Instead, mutate the existing renderer's color profile in-place by detecting
		// capabilities from /dev/tty before stdout/stderr are redirected to /dev/null.
		ttyOut := termenv.NewOutput(tty)
		lipgloss.SetColorProfile(ttyOut.ColorProfile())
		lipgloss.SetHasDarkBackground(ttyOut.HasDarkBackground())

		restoreFds := muteCLibOutput()
		p := tea.NewProgram(newTUIModel(cfg),
			tea.WithAltScreen(),
			tea.WithInput(tty),
			tea.WithOutput(tty),
		)
		finalModel, err := p.Run()
		restoreFds()
		tty.Close()
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		if m, ok := finalModel.(tuiModel); ok {
			for _, s := range m.steps {
				if s.err != nil {
					return fmt.Errorf("%s: %w", filepath.Base(inputPath), s.err)
				}
			}
		}
	}
	return nil
}

// expandDir returns all supported video/audio files in dir (non-recursive).
func expandDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if videoExts[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}
