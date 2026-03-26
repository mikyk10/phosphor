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

type RemixUsecase interface {
	Run(ctx context.Context, input RemixInput) (*RemixOutput, error)
}

type RemixInput struct {
	PipelineName string
	SourceImage  []byte
	Width        int
	Height       int
	Quality      string
}

type RemixOutput struct {
	ImageData   []byte
	ContentType string
}

type remixUsecase struct {
	svcCfg *config.ServiceConfig
	runner *PipelineRunner
	repo   store.Repository
}

func NewRemixUsecase(svcCfg *config.ServiceConfig, runner *PipelineRunner, repo store.Repository) RemixUsecase {
	return &remixUsecase{svcCfg: svcCfg, runner: runner, repo: repo}
}

func (u *remixUsecase) Run(ctx context.Context, input RemixInput) (*RemixOutput, error) {
	pipelineName := input.PipelineName
	if pipelineName == "" {
		pipelineName = "remix"
	}

	pipelineCfg, ok := u.svcCfg.Pipelines[pipelineName]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", pipelineName)
	}

	if err := validateLastStageOutput(pipelineCfg, "image"); err != nil {
		return nil, err
	}

	d := pipelineCfg.Defaults
	width := withDefault(input.Width, d.Width)
	height := withDefault(input.Height, d.Height)
	quality := withDefaultStr(input.Quality, d.Quality)

	exec := &store.PipelineExecution{
		PipelineName: pipelineName,
		Status:       store.StatusRunning,
		StartedAt:    time.Now(),
	}
	if err := u.repo.CreatePipelineExecution(exec); err != nil {
		slog.Warn("failed to record pipeline execution", "err", err)
	}

	size := ""
	if width > 0 && height > 0 {
		size = fmt.Sprintf("%dx%d", width, height)
	}

	result, err := u.runner.RunPipeline(ctx, RunPipelineInput{
		PipelineExecID: exec.ID,
		Stages:         pipelineCfg.Stages,
		SourceImage:    input.SourceImage,
		ConfigVars: map[string]any{
			"Width":   width,
			"Height":  height,
			"Quality": quality,
		},
		ExecutionParams: pipeline.ExecutionParams{
			Size:    size,
			Quality: quality,
		},
	})

	if err != nil {
		exec.Status = store.StatusFailed
		exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
		if err := u.repo.UpdatePipelineExecution(exec); err != nil {
			slog.Warn("failed to update pipeline execution", "err", err)
		}
		return nil, err
	}

	exec.Status = store.StatusSuccess
	exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if err := u.repo.UpdatePipelineExecution(exec); err != nil {
			slog.Warn("failed to update pipeline execution", "err", err)
		}

	imgData, ct := result.LastImageOutput()
	if imgData == nil {
		return nil, fmt.Errorf("pipeline %q produced no image output", pipelineName)
	}

	return &RemixOutput{ImageData: imgData, ContentType: ct}, nil
}
