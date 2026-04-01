package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:          "subgolem -i <input> [-o <output.srt>]",
		Short:        "Generate English SRT subtitles from non-English video",
		Long:         "subgolem transcribes and translates video audio to English SRT subtitles using a local whisper.cpp model.",
		RunE:         run,
		SilenceUsage: true,
	}

	cobra.OnInitialize(func() { initConfig(cfgFile) })

	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config.yaml)")
	cmd.Flags().StringP("input", "i", "", "input video or audio file")
	cmd.Flags().StringP("output", "o", "", "output SRT file (default: <input>.srt)")
	cmd.Flags().String("model", "large-v3", "whisper model: tiny|base|small|medium|large-v3")
	cmd.Flags().String("language", "auto", "source language code (e.g. 'he') or 'auto'")
	cmd.Flags().String("translator", "whisper", "translation backend: whisper|openai")
	cmd.Flags().Bool("gpu", true, "enable Vulkan GPU acceleration")
	cmd.Flags().String("data-dir", "data", "directory for models and temp files")

	viper.BindPFlag("input", cmd.Flags().Lookup("input"))
	viper.BindPFlag("output", cmd.Flags().Lookup("output"))
	viper.BindPFlag("model", cmd.Flags().Lookup("model"))
	viper.BindPFlag("language", cmd.Flags().Lookup("language"))
	viper.BindPFlag("translator", cmd.Flags().Lookup("translator"))
	viper.BindPFlag("gpu", cmd.Flags().Lookup("gpu"))
	viper.BindPFlag("data_dir", cmd.Flags().Lookup("data-dir"))

	return cmd
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

func run(_ *cobra.Command, _ []string) error {
	inputPath := viper.GetString("input")
	if inputPath == "" {
		return fmt.Errorf("input file required — use -i flag or set 'input' in config.yaml")
	}
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file not found: %s", inputPath)
	}

	outputPath := viper.GetString("output")
	if outputPath == "" {
		ext := filepath.Ext(inputPath)
		outputPath = strings.TrimSuffix(inputPath, ext) + ".srt"
	}

	translatorID := viper.GetString("translator")
	if translatorID != "whisper" && translatorID != "openai" {
		return fmt.Errorf("unknown translator %q — valid: whisper, openai", translatorID)
	}

	cfg := PipelineConfig{
		InputPath:     inputPath,
		OutputPath:    outputPath,
		ModelName:     viper.GetString("model"),
		Lang:          viper.GetString("language"),
		TranslatorID:  translatorID,
		DataDir:       viper.GetString("data_dir"),
		OpenAIBaseURL: viper.GetString("openai.base_url"),
		OpenAIAPIKey:  viper.GetString("openai.api_key"),
		OpenAIModel:   viper.GetString("openai.model"),
	}

	p := tea.NewProgram(newTUIModel(cfg), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Surface any pipeline error that caused the TUI to quit
	if m, ok := finalModel.(tuiModel); ok {
		for _, s := range m.steps {
			if s.err != nil {
				return s.err
			}
		}
	}
	return nil
}
