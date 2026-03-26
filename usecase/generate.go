package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikyk10/wisp-ai/config"
	"github.com/mikyk10/wisp-ai/pipeline"
	"github.com/mikyk10/wisp-ai/store"
)

type GenerateUsecase interface {
	Run(ctx context.Context, input GenerateInput) (*GenerateOutput, error)
}

type GenerateInput struct {
	PipelineName string
	Width        int
	Height       int
	Orientation  string
	Quality      string
}

type GenerateOutput struct {
	ImageData   []byte
	ContentType string
}

type generateUsecase struct {
	svcCfg *config.ServiceConfig
	runner *PipelineRunner
	repo   store.Repository
}

func NewGenerateUsecase(svcCfg *config.ServiceConfig, runner *PipelineRunner, repo store.Repository) GenerateUsecase {
	return &generateUsecase{svcCfg: svcCfg, runner: runner, repo: repo}
}

func (u *generateUsecase) Run(ctx context.Context, input GenerateInput) (*GenerateOutput, error) {
	pipelineName := input.PipelineName
	if pipelineName == "" {
		pipelineName = "generate"
	}

	pipelineCfg, ok := u.svcCfg.Pipelines[pipelineName]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", pipelineName)
	}

	if err := validateLastStageOutput(pipelineCfg, "image"); err != nil {
		return nil, err
	}

	// Apply defaults from pipeline config where request params are missing.
	d := pipelineCfg.Defaults
	width := withDefault(input.Width, d.Width)
	height := withDefault(input.Height, d.Height)
	orientation := withDefaultStr(input.Orientation, d.Orientation)
	quality := withDefaultStr(input.Quality, d.Quality)

	slog.Debug("usecase: generate", "pipeline", pipelineName, "width", width, "height", height, "orientation", orientation, "quality", quality)

	exec := &store.PipelineExecution{
		PipelineName: pipelineName,
		Status:       store.StatusRunning,
		StartedAt:    time.Now(),
	}
	_ = u.repo.CreatePipelineExecution(exec)

	// Build size string from width/height if both are provided.
	size := ""
	if width > 0 && height > 0 {
		size = fmt.Sprintf("%dx%d", width, height)
	}

	result, err := u.runner.RunPipeline(ctx, RunPipelineInput{
		PipelineExecID: exec.ID,
		Stages:         pipelineCfg.Stages,
		ConfigVars: map[string]any{
			"Width":       width,
			"Height":      height,
			"Orientation": orientation,
			"Quality":     quality,
		},
		ExecutionParams: pipeline.ExecutionParams{
			Size:        size,
			Quality:     quality,
			Orientation: orientation,
		},
	})

	if err != nil {
		exec.Status = store.StatusFailed
		exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
		_ = u.repo.UpdatePipelineExecution(exec)
		return nil, err
	}

	exec.Status = store.StatusSuccess
	exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	_ = u.repo.UpdatePipelineExecution(exec)

	imgData, ct := result.LastImageOutput()
	if imgData == nil {
		return nil, fmt.Errorf("pipeline %q produced no image output", pipelineName)
	}

	return &GenerateOutput{ImageData: imgData, ContentType: ct}, nil
}

func withDefault(val, fallback int) int {
	if val != 0 {
		return val
	}
	return fallback
}

func withDefaultStr(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

func validateLastStageOutput(cfg config.PipelineConfig, expected string) error {
	if len(cfg.Stages) == 0 {
		return fmt.Errorf("pipeline has no stages")
	}
	last := cfg.Stages[len(cfg.Stages)-1]
	if last.Output != expected {
		return fmt.Errorf("pipeline last stage outputs %q, expected %q", last.Output, expected)
	}
	return nil
}
