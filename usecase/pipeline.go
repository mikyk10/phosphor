package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/mikyk10/phosphor/config"
	"github.com/mikyk10/phosphor/pipeline"
	"github.com/mikyk10/phosphor/store"
)

type PipelineUsecase interface {
	Run(ctx context.Context, input PipelineInput) (*PipelineOutput, error)
}

type PipelineInput struct {
	PipelineName string
	SourceImage  []byte // nil for text-to-image
	Size         string
	Quality      string
	MaxTags      int
}

type PipelineOutput struct {
	OutputType  string   // "image" or "text"
	ImageData   []byte   // non-nil when OutputType == "image"
	ContentType string   // MIME type for image
	Text        string   // raw text when OutputType == "text" and not tags
	Tags        []string // parsed tags when last stage is text
}

type pipelineUsecase struct {
	svcCfg *config.ServiceConfig
	runner *PipelineRunner
	repo   store.Repository
}

func NewPipelineUsecase(svcCfg *config.ServiceConfig, runner *PipelineRunner, repo store.Repository) PipelineUsecase {
	return &pipelineUsecase{svcCfg: svcCfg, runner: runner, repo: repo}
}

func (u *pipelineUsecase) Run(ctx context.Context, input PipelineInput) (*PipelineOutput, error) {
	if input.PipelineName == "" {
		return nil, fmt.Errorf("pipeline name is required")
	}

	pipelineCfg, ok := u.svcCfg.Pipelines[input.PipelineName]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", input.PipelineName)
	}

	if len(pipelineCfg.Stages) == 0 {
		return nil, fmt.Errorf("pipeline %q has no stages", input.PipelineName)
	}

	d := pipelineCfg.Defaults
	size := withDefaultStr(input.Size, d.Size)
	quality := withDefaultStr(input.Quality, d.Quality)
	maxTags := withDefaultInt(input.MaxTags, d.MaxTags)
	if maxTags <= 0 {
		maxTags = 10
	}

	lastStageOutput := pipelineCfg.Stages[len(pipelineCfg.Stages)-1].Output

	slog.Debug("usecase: pipeline", "name", input.PipelineName, "size", size, "quality", quality, "output", lastStageOutput)

	exec := &store.PipelineExecution{
		PipelineName: input.PipelineName,
		Status:       store.StatusRunning,
		StartedAt:    time.Now(),
	}
	if err := u.repo.CreatePipelineExecution(exec); err != nil {
		slog.Warn("failed to record pipeline execution", "err", err)
	}

	result, err := u.runner.RunPipeline(ctx, RunPipelineInput{
		PipelineExecID: exec.ID,
		Stages:         pipelineCfg.Stages,
		SourceImage:    input.SourceImage,
		ConfigVars: map[string]any{
			"Size":    size,
			"Quality": quality,
			"MaxTags": maxTags,
		},
		ExecutionParams: pipeline.ExecutionParams{
			Size:    size,
			Quality: quality,
		},
	})

	if err != nil {
		exec.Status = store.StatusFailed
		exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
		if dbErr := u.repo.UpdatePipelineExecution(exec); dbErr != nil {
			slog.Warn("failed to update pipeline execution", "err", dbErr)
		}
		return nil, err
	}

	exec.Status = store.StatusSuccess
	exec.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if dbErr := u.repo.UpdatePipelineExecution(exec); dbErr != nil {
		slog.Warn("failed to update pipeline execution", "err", dbErr)
	}

	// Determine response based on last stage output type.
	switch lastStageOutput {
	case "image":
		imgData, ct := result.LastImageOutput()
		if imgData == nil {
			return nil, fmt.Errorf("pipeline %q produced no image output", input.PipelineName)
		}
		return &PipelineOutput{
			OutputType:  "image",
			ImageData:   imgData,
			ContentType: ct,
		}, nil
	case "text":
		rawText := result.LastTextOutput()
		tags := parseTags(rawText, maxTags)
		return &PipelineOutput{
			OutputType: "text",
			Text:       rawText,
			Tags:       tags,
		}, nil
	default:
		return nil, fmt.Errorf("pipeline %q: unknown output type %q", input.PipelineName, lastStageOutput)
	}
}

func withDefaultStr(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

func withDefaultInt(val, fallback int) int {
	if val != 0 {
		return val
	}
	return fallback
}

// parseTags extracts normalized tags from LLM text output.
func parseTags(text string, maxTags int) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	})

	seen := make(map[string]bool)
	var tags []string

	for _, raw := range fields {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" || !isAlphaOnly(tag) || len(tag) < 2 {
			continue
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
		if len(tags) >= maxTags {
			break
		}
	}
	return tags
}

func isAlphaOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
