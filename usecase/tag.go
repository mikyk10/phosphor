package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/mikyk10/wisp-ai/config"
	"github.com/mikyk10/wisp-ai/store"
)

type TagUsecase interface {
	Run(ctx context.Context, input TagInput) (*TagOutput, error)
}

type TagInput struct {
	PipelineName string
	Image        []byte
	MaxTags      int
}

type TagOutput struct {
	Tags []string
}

type tagUsecase struct {
	svcCfg *config.ServiceConfig
	runner *PipelineRunner
	repo   store.Repository
}

func NewTagUsecase(svcCfg *config.ServiceConfig, runner *PipelineRunner, repo store.Repository) TagUsecase {
	return &tagUsecase{svcCfg: svcCfg, runner: runner, repo: repo}
}

func (u *tagUsecase) Run(ctx context.Context, input TagInput) (*TagOutput, error) {
	pipelineName := input.PipelineName
	if pipelineName == "" {
		pipelineName = "tag"
	}

	pipelineCfg, ok := u.svcCfg.Pipelines[pipelineName]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", pipelineName)
	}

	if err := validateLastStageOutput(pipelineCfg, "text"); err != nil {
		return nil, err
	}

	d := pipelineCfg.Defaults
	maxTags := withDefaultInt(input.MaxTags, d.MaxTags)
	if maxTags <= 0 {
		maxTags = 10
	}

	exec := &store.PipelineExecution{
		PipelineName: pipelineName,
		Status:       store.StatusRunning,
		StartedAt:    time.Now(),
	}
	if err := u.repo.CreatePipelineExecution(exec); err != nil {
		slog.Warn("failed to record pipeline execution", "err", err)
	}

	result, err := u.runner.RunPipeline(ctx, RunPipelineInput{
		PipelineExecID: exec.ID,
		Stages:         pipelineCfg.Stages,
		SourceImage:    input.Image,
		ConfigVars: map[string]any{
			"MaxTags": maxTags,
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

	rawText := result.LastTextOutput()
	tags := parseTags(rawText, maxTags)

	return &TagOutput{Tags: tags}, nil
}

// parseTags extracts normalized tags from LLM text output.
func parseTags(text string, maxTags int) []string {
	// Split by whitespace, commas, or newlines.
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	})

	seen := make(map[string]bool)
	var tags []string

	for _, raw := range fields {
		tag := strings.ToLower(strings.TrimSpace(raw))
		// Only keep alphabetic tags (a-z), skip numbers, punctuation, etc.
		if tag == "" || !isAlphaOnly(tag) {
			continue
		}
		if len(tag) < 2 {
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
