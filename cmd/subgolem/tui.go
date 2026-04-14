package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/roarc0/subgolem/internal/audio"
	"github.com/roarc0/subgolem/internal/models"
	"github.com/roarc0/subgolem/internal/refine"
	intsegment "github.com/roarc0/subgolem/internal/segment"
	"github.com/roarc0/subgolem/internal/subtitle"
	"github.com/roarc0/subgolem/internal/transcribe"
)

// ── pipeline step indices ──────────────────────────────────────────────────

const (
	stepExtract = iota
	stepDownload
	stepTranscribe
	stepWrite
	stepRefine
	numSteps
)

var stepLabels = [numSteps]string{
	"Extracting audio",
	"Downloading whisper model",
	"Transcribing",
	"Writing subtitles",
	"Translating and refining subtitles",
}

// ── status ─────────────────────────────────────────────────────────────────

type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusDone
	statusFailed
)

type stepState struct {
	status stepStatus
	info   string
	err    error
}

// ── messages ───────────────────────────────────────────────────────────────

type stepDoneMsg struct {
	idx  int
	info string
}

type stepErrMsg struct {
	idx int
	err error
}

type dlProgressMsg struct {
	done  int64
	total int64
}

type exProgressMsg struct{ pct float32 }
type txProgressMsg struct {
	pct         float32
	chunk       int
	totalChunks int
}
type rfProgressMsg struct {
	chunk       int
	totalChunks int
}

// rfProgressCh streams LLM refine chunk progress into the TUI event loop.
type rfProgressCh chan rfProgressMsg

func (ch rfProgressCh) wait() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// exProgressCh streams audio extraction progress into the TUI event loop.
type exProgressCh chan exProgressMsg

func (ch exProgressCh) wait() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// dlProgressCh streams download progress into the TUI event loop.
type dlProgressCh chan dlProgressMsg

func (ch dlProgressCh) wait() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// txProgressCh streams transcription progress into the TUI event loop.
type txProgressCh chan txProgressMsg

func (ch txProgressCh) wait() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// ── config ─────────────────────────────────────────────────────────────────

// PipelineConfig holds all runtime configuration for a run.
type PipelineConfig struct {
	InputPath      string
	OutputPath     string
	ModelName      string
	Lang           string
	DataDir        string
	AudioFilter    bool
	ChunkSize      int
	Prompt         string
	BeamSize       int
	VAD            bool
	Clean          bool
	MergeGap       time.Duration // 0 = no merging
	MergeChars     int
	SplitChars     int // 0 = disabled
	FixOverlaps    bool
	FileIndex      int // 1-based index when processing multiple files (0 = single file)
	FileCount      int
	RefinerEnabled bool
	RefinerBaseURL string
	RefinerAPIKey  string
	RefinerModel   string
	RefinerChunk   int
	RefinerPrompt  string
	WhisperModels  map[string]string
}

// ── shared pipeline state ──────────────────────────────────────────────────

type pipeState struct {
	segments   []intsegment.Segment
	sourceLang string
	cancel     context.CancelFunc
}

// ── styles ─────────────────────────────────────────────────────────────────

var (
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleBlue  = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	styleTitle = lipgloss.NewStyle().Bold(true)
	styleMeta  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// ── TUI model ──────────────────────────────────────────────────────────────

type tuiModel struct {
	cfg     PipelineConfig
	steps   [numSteps]stepState
	current int
	spinner spinner.Model
	prog    progress.Model // reused for all progress bars
	dlCh    dlProgressCh
	dlPct   float64
	dlDone  int64
	dlTotal int64
	exCh    exProgressCh
	exPct   float64
	txCh    txProgressCh
	txPct   float64
	txChunk int
	txTotal int
	rfCh    rfProgressCh
	rfChunk int
	rfTotal int
	rfPct   float64
	pipe    *pipeState // shared across bubbletea value copies
	mgr     *models.Manager
	done    bool
}

func newTUIModel(cfg PipelineConfig) tuiModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styleBlue

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	return tuiModel{
		cfg:     cfg,
		spinner: s,
		prog:    p,
		dlCh:    make(dlProgressCh, 256),
		exCh:    make(exProgressCh, 256),
		txCh:    make(txProgressCh, 256),
		rfCh:    make(rfProgressCh, 256),
		pipe:    &pipeState{},
		mgr:     models.NewManager(cfg.DataDir, cfg.WhisperModels),
	}
}

func (m tuiModel) Init() tea.Cmd {
	m.steps[stepExtract].status = statusRunning
	return tea.Batch(m.spinner.Tick, m.cmdExtract())
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			if m.pipe.cancel != nil {
				m.pipe.cancel()
			}
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case dlProgressMsg:
		m.dlDone = msg.done
		m.dlTotal = msg.total
		if msg.total > 0 {
			m.dlPct = float64(msg.done) / float64(msg.total)
		} else {
			// If total known is 0, we treat it as infinite for the bar but we still want to show bytes
			m.dlPct = 0
		}
		progCmd := m.prog.SetPercent(m.dlPct)
		return m, tea.Batch(progCmd, m.dlCh.wait())

	case exProgressMsg:
		m.exPct = float64(msg.pct)
		progCmd := m.prog.SetPercent(m.exPct)
		return m, tea.Batch(progCmd, m.exCh.wait())

	case txProgressMsg:
		m.txPct = float64(msg.pct)
		m.txChunk = msg.chunk
		m.txTotal = msg.totalChunks
		progCmd := m.prog.SetPercent(m.txPct)
		return m, tea.Batch(progCmd, m.txCh.wait())

	case rfProgressMsg:
		m.rfChunk = msg.chunk
		m.rfTotal = msg.totalChunks
		if m.rfTotal > 0 {
			m.rfPct = float64(m.rfChunk) / float64(m.rfTotal)
		}
		progCmd := m.prog.SetPercent(m.rfPct)
		return m, tea.Batch(progCmd, m.rfCh.wait())

	case progress.FrameMsg:
		newProg, cmd := m.prog.Update(msg)
		m.prog = newProg.(progress.Model)
		return m, cmd

	case stepDoneMsg:
		m.steps[msg.idx].status = statusDone
		m.steps[msg.idx].info = msg.info
		next := msg.idx + 1

		if next == stepRefine && !m.cfg.RefinerEnabled {
			next++
		}

		if next >= numSteps {
			m.done = true
			return m, tea.Quit
		}
		if next < numSteps {
			m.current = next
			m.steps[next].status = statusRunning
			// Reset progress bar for next step
			m.dlPct = 0
			m.exPct = 0
			m.txPct = 0
			m.txCh = make(txProgressCh, 256)
		}
		// reset progress bar to 0 for the incoming step
		progCmd, _ := m.prog.Update(m.prog.SetPercent(0))
		m.prog = progCmd.(progress.Model)
		return m, m.cmdForStep(next)

	case stepErrMsg:
		m.steps[msg.idx].status = statusFailed
		m.steps[msg.idx].err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m tuiModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	title := "subgolem"
	if m.cfg.FileCount > 1 {
		title += fmt.Sprintf("  %s", styleMeta.Render(fmt.Sprintf("[%d/%d]", m.cfg.FileIndex, m.cfg.FileCount)))
	}
	b.WriteString(styleTitle.Render(title) + "\n\n")

	meta := fmt.Sprintf("  %s  →  %s   model: %s · lang: %s",
		styleMeta.Render(filepath.Base(m.cfg.InputPath)),
		styleMeta.Render(filepath.Base(m.cfg.OutputPath)),
		styleMeta.Render(m.cfg.ModelName),
		styleMeta.Render(m.cfg.Lang),
	)
	b.WriteString(meta + "\n\n")

	for i, s := range m.steps {
		var icon, label string
		switch s.status {
		case statusPending:
			icon = styleDim.Render("○")
			label = styleDim.Render(stepLabels[i])
		case statusRunning:
			icon = m.spinner.View()
			label = stepLabels[i]
			switch i {
			case stepDownload:
				if m.dlDone > 0 {
					label += styleDim.Render(fmt.Sprintf("  %.1f MB", float64(m.dlDone)/1024/1024))
					if m.dlTotal > 0 {
						label += styleDim.Render(fmt.Sprintf(" / %.1f MB", float64(m.dlTotal)/1024/1024))
					} else {
						label += styleDim.Render(" / unknown size")
					}
				} else {
					label += styleDim.Render("  connecting…")
				}
			case stepTranscribe:
				if m.txTotal > 0 {
					label += styleDim.Render(fmt.Sprintf("  (chunk %d/%d)", m.txChunk, m.txTotal))
				}
			case stepRefine:
				host := m.cfg.RefinerBaseURL
				switch host {
				case "http://localhost:11434/v1":
					host = "ollama"
				case "http://localhost:8080/v1":
					host = "llama.cpp"
				}
				label += styleDim.Render(fmt.Sprintf("  %s @ %s", m.cfg.RefinerModel, host))
				if m.rfTotal > 0 {
					label += styleDim.Render(fmt.Sprintf("  (chunk %d/%d)", m.rfChunk, m.rfTotal))
				} else {
					label += styleDim.Render("  waiting for first chunk…")
				}
			}
		case statusDone:
			icon = styleGreen.Render("✓")
			label = stepLabels[i]
			if s.info != "" {
				label += styleDim.Render("  " + s.info)
			}
		case statusFailed:
			icon = styleRed.Render("✗")
			label = styleRed.Render(stepLabels[i])
			if s.err != nil {
				label += "\n     " + styleRed.Render(s.err.Error())
			}
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", icon, label))

		if s.status == statusRunning && (i == stepDownload || i == stepExtract || i == stepTranscribe || i == stepRefine) {
			b.WriteString("\n  " + m.prog.View() + "\n\n")
		}
	}

	if m.done {
		b.WriteString("\n" + styleGreen.Render("  ✓  Saved original: "+m.cfg.OutputPath) + "\n")
		if m.cfg.RefinerEnabled {
			ext := filepath.Ext(m.cfg.OutputPath)
			fixedPath := strings.TrimSuffix(m.cfg.OutputPath, ext) + "_fixed" + ext
			b.WriteString(styleGreen.Render("  ✓  Saved refined:  "+fixedPath) + "\n")
		}
		b.WriteString("\n  Done!\n")
	}

	b.WriteString("\n")
	return b.String()
}

// cmdForStep dispatches to the right pipeline command for step idx.
func (m tuiModel) cmdForStep(idx int) tea.Cmd {
	switch idx {
	case stepExtract:
		return m.cmdExtract()
	case stepDownload:
		return m.cmdDownload()
	case stepTranscribe:
		return m.cmdTranscribe()
	case stepWrite:
		return m.cmdWrite()
	case stepRefine:
		return m.cmdRefine()
	}
	return nil
}

// ── pipeline commands ───────────────────────────────────────────────────────

func (m tuiModel) cmdDownload() tea.Cmd {
	ch := m.dlCh
	mgr := m.mgr
	pipe := m.pipe
	modelName := m.cfg.ModelName

	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			pipe.cancel = cancel
			defer cancel()

			err := mgr.EnsureDownloaded(ctx, modelName, func(done, total int64) {
				select {
				case ch <- dlProgressMsg{done, total}:
				default:
				}
			})
			close(ch)
			if err != nil {
				return stepErrMsg{stepDownload, err}
			}
			return stepDoneMsg{stepDownload, modelName}
		},
		ch.wait(),
	)
}

func (m tuiModel) cmdExtract() tea.Cmd {
	cfg := m.cfg
	pipe := m.pipe
	ch := m.exCh

	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			pipe.cancel = cancel
			defer cancel()

			tmpDir := filepath.Join(cfg.DataDir, "tmp")
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				close(ch)
				return stepErrMsg{stepExtract, err}
			}

			pcmPath := filepath.Join(tmpDir, "audio.pcm")
			err := audio.NewExtractor(cfg.AudioFilter).Extract(ctx, cfg.InputPath, pcmPath, func(done, total time.Duration) {
				if total > 0 {
					pct := float32(done) / float32(total)
					if pct > 1 {
						pct = 1
					}
					select {
					case ch <- exProgressMsg{pct}:
					default:
					}
				}
			})
			close(ch)
			if err != nil {
				return stepErrMsg{stepExtract, err}
			}
			return stepDoneMsg{stepExtract, ""}
		},
		ch.wait(),
	)
}

func (m tuiModel) cmdTranscribe() tea.Cmd {
	cfg := m.cfg
	mgr := m.mgr
	pipe := m.pipe
	ch := m.txCh

	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			pipe.cancel = cancel
			defer cancel()

			pcmPath := filepath.Join(cfg.DataDir, "tmp", "audio.pcm")

			t, err := transcribe.NewWhisperTranscriber(mgr.ModelPath(cfg.ModelName), cfg.BeamSize, cfg.VAD, cfg.Prompt, cfg.ChunkSize)
			if err != nil {
				close(ch)
				return stepErrMsg{stepTranscribe, err}
			}
			defer t.Close()

			segs, detectedLang, err := t.Transcribe(ctx, pcmPath, cfg.Lang, false, func(pct float32, chunk int, total int) {
				select {
				case ch <- txProgressMsg{pct, chunk, total}:
				default:
				}
			})
			close(ch)
			if err != nil {
				return stepErrMsg{stepTranscribe, err}
			}
			pipe.sourceLang = detectedLang

			if cfg.Clean {
				segs = intsegment.Clean(segs)
			}
			if cfg.MergeGap > 0 {
				segs = intsegment.Merge(segs, cfg.MergeGap, cfg.MergeChars)
			}
			if cfg.SplitChars > 0 {
				segs = intsegment.Split(segs, cfg.SplitChars)
			}
			if cfg.FixOverlaps {
				segs = intsegment.FixOverlaps(segs)
			}

			pipe.segments = segs
			return stepDoneMsg{stepTranscribe, fmt.Sprintf("%d segments", len(segs))}
		},
		ch.wait(),
	)
}

func (m tuiModel) cmdWrite() tea.Cmd {
	cfg := m.cfg
	pipe := m.pipe

	return func() tea.Msg {
		if err := subtitle.WriteSRT(cfg.OutputPath, pipe.segments); err != nil {
			return stepErrMsg{stepWrite, err}
		}
		return stepDoneMsg{stepWrite, cfg.OutputPath}
	}
}

func (m *tuiModel) cmdRefine() tea.Cmd {
	cfg := m.cfg
	pipe := m.pipe
	rfCh := m.rfCh

	runFn := func() tea.Msg {
		rCfg := refine.RefineConfig{
			BaseURL:    cfg.RefinerBaseURL,
			APIKey:     cfg.RefinerAPIKey,
			Model:      cfg.RefinerModel,
			Prompt:     cfg.RefinerPrompt,
			SourceLang: pipe.sourceLang,
			Chunk:      cfg.RefinerChunk,
			OnProgress: func(chunk, total int) {
				rfCh <- rfProgressMsg{chunk: chunk, totalChunks: total}
			},
		}

		r := refine.NewLlmRefiner(rCfg)
		refinedStr, err := r.Refine(context.Background(), pipe.segments)
		close(rfCh)
		if err != nil {
			return stepErrMsg{stepRefine, err}
		}

		ext := filepath.Ext(cfg.OutputPath)
		outPath := strings.TrimSuffix(cfg.OutputPath, ext) + "_fixed" + ext
		if ext == "" {
			outPath = cfg.OutputPath + "_fixed.srt"
		}

		if err := os.WriteFile(outPath, []byte(refinedStr), 0644); err != nil {
			return stepErrMsg{stepRefine, err}
		}

		return stepDoneMsg{stepRefine, outPath}
	}

	return tea.Batch(runFn, rfCh.wait())
}
