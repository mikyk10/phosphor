package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikyk10/phosphor/config"
	"github.com/mikyk10/phosphor/llm"
	"github.com/mikyk10/phosphor/pipeline"
	"github.com/mikyk10/phosphor/store"
)

// ExecutorFactory creates a StageExecutor from prompt metadata and output type.
type ExecutorFactory func(providers map[string]config.ProviderConfig, meta llm.PromptMeta, outputType string, timeout time.Duration, params pipeline.ExecutionParams) (pipeline.StageExecutor, error)

// PipelineRunner executes a pipeline definition for a single input,
// recording each step in the database.
type PipelineRunner struct {
	cfg             *config.GlobalConfig
	repo            store.Repository
	executorFactory ExecutorFactory
}

func NewPipelineRunner(cfg *config.GlobalConfig, repo store.Repository) *PipelineRunner {
	return &PipelineRunner{cfg: cfg, repo: repo, executorFactory: llm.NewStageExecutor}
}

// RunPipelineInput holds the inputs for a pipeline execution.
type RunPipelineInput struct {
	PipelineExecID  store.PrimaryKey
	Stages          []config.StageConfig
	SourceImage     []byte                // _source image data (may be nil)
	ConfigVars      map[string]any        // template config variables
	ExecutionParams pipeline.ExecutionParams // runtime overrides for size/quality
}

// RunPipeline executes all stages of a pipeline sequentially.
func (r *PipelineRunner) RunPipeline(ctx context.Context, input RunPipelineInput) (*pipeline.PipelineResult, error) {
	result := &pipeline.PipelineResult{}
	stageOutputs := make(map[string]llm.StageOutput)

	timeout := time.Duration(r.cfg.AI.RequestTimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	maxRetries := r.cfg.AI.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for i, stage := range input.Stages {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		slog.Debug("runner: executing stage", "stage", stage.Name, "index", i, "output", stage.Output, "image_input", stage.ImageInput)

		if stage.Prompt == "" {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "prompt_load_failed",
				fmt.Errorf("stage %q has no prompt path configured", stage.Name))
		}
		prompt, err := llm.LoadPrompt(stage.Prompt)
		if err != nil {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "prompt_load_failed", err)
		}

		var prev llm.StageOutput
		if i > 0 {
			prev = stageOutputs[input.Stages[i-1].Name]
		}

		renderedPrompt, err := llm.RenderPrompt(prompt.Body, llm.TemplateData{
			Prev:   prev,
			Stages: stageOutputs,
			Config: input.ConfigVars,
		})
		if err != nil {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "prompt_render_failed", err)
		}

		var imageInputs [][]byte
		if stage.ImageInput != "" {
			imgData, err := resolveImageInput(stage.ImageInput, input.SourceImage, stageOutputs)
			if err != nil {
				return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "image_input_failed", err)
			}
			if imgData != nil {
				imageInputs = append(imageInputs, imgData)
			}
		}

		executor, err := r.executorFactory(r.cfg.AI.Providers, prompt.Meta, stage.Output, timeout, input.ExecutionParams)
		if err != nil {
			return nil, r.recordStepFailure(input.PipelineExecID, stage, i, "executor_create_failed", err)
		}

		step := &store.StepExecution{
			PipelineExecutionID: input.PipelineExecID,
			StageName:           stage.Name,
			StageIndex:          i,
			ProviderName:        prompt.Meta.Provider,
			ModelName:           prompt.Meta.Model,
			PromptHash:          prompt.Hash,
			Status:              store.StatusRunning,
			StartedAt:           time.Now(),
		}
		if err := r.repo.CreateStepExecution(step); err != nil {
			return nil, fmt.Errorf("create step execution: %w", err)
		}

		var sr *pipeline.StageResult
		var execErr error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if ctx.Err() != nil {
				r.failStep(step, "context_cancelled", ctx.Err())
				return nil, ctx.Err()
			}

			start := time.Now()
			sr, execErr = executor.Execute(ctx, renderedPrompt, imageInputs)
			step.LatencyMs = time.Since(start).Milliseconds()

			if execErr == nil {
				break
			}

			if ctx.Err() != nil {
				r.failStep(step, "context_cancelled", ctx.Err())
				return nil, ctx.Err()
			}

			if attempt < maxRetries {
				backoff := time.Duration(1<<uint(attempt)) * time.Second
				slog.Warn("pipeline: stage failed, retrying", "stage", stage.Name, "attempt", attempt+1, "err", execErr, "backoff", backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					r.failStep(step, "context_cancelled", ctx.Err())
					return nil, ctx.Err()
				}
			}
		}

		step.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}

		if execErr != nil {
			r.failStep(step, "execution_failed", execErr)
			return nil, fmt.Errorf("stage %q failed: %w", stage.Name, execErr)
		}

		step.Status = store.StatusSuccess
		if err := r.repo.UpdateStepExecution(step); err != nil {
			slog.Warn("failed to update step execution", "stage", stage.Name, "err", err)
		}

		sr.StageName = stage.Name
		output := &store.StepOutput{
			StepExecutionID: step.ID,
			ContentType:     "text/plain",
		}
		if sr.OutputType == "text" {
			output.ContentText = &sr.Text
		} else {
			output.ContentBlob = sr.ImageData
			output.ContentType = sr.ContentType
		}
		if err := r.repo.CreateStepOutput(output); err != nil {
			slog.Warn("failed to save step output", "stage", stage.Name, "err", err)
		}

		result.Stages = append(result.Stages, *sr)
		stageOutputs[stage.Name] = llm.StageOutput{
			Text:  sr.Text,
			Image: sr.ImageData,
		}

		attrs := []any{"stage", stage.Name, "output_type", sr.OutputType, "latency_ms", step.LatencyMs}
		if sr.OutputType == "text" {
			attrs = append(attrs, "output", sr.Text)
		} else {
			attrs = append(attrs, "output_bytes", len(sr.ImageData))
		}
		slog.Info("runner: stage completed", attrs...)
	}

	return result, nil
}

func resolveImageInput(ref string, sourceImage []byte, stageOutputs map[string]llm.StageOutput) ([]byte, error) {
	if ref == "_source" {
		if sourceImage == nil {
			return nil, fmt.Errorf("_source referenced but no source image provided")
		}
		return sourceImage, nil
	}
	out, ok := stageOutputs[ref]
	if !ok {
		return nil, fmt.Errorf("image_input references unknown stage %q", ref)
	}
	if out.Image == nil {
		return nil, fmt.Errorf("stage %q has no image output", ref)
	}
	return out.Image, nil
}

func (r *PipelineRunner) failStep(step *store.StepExecution, errorCode string, err error) {
	step.Status = store.StatusFailed
	step.ErrorCode = errorCode
	step.ErrorMessage = err.Error()
	step.FinishedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if dbErr := r.repo.UpdateStepExecution(step); dbErr != nil {
		slog.Warn("failed to record step failure", "stage", step.StageName, "err", dbErr)
	}
}

func (r *PipelineRunner) recordStepFailure(pipelineExecID store.PrimaryKey, stage config.StageConfig, index int, errorCode string, err error) error {
	step := &store.StepExecution{
		PipelineExecutionID: pipelineExecID,
		StageName:           stage.Name,
		StageIndex:          index,
		Status:              store.StatusFailed,
		StartedAt:           time.Now(),
		FinishedAt:          sql.NullTime{Time: time.Now(), Valid: true},
		ErrorCode:           errorCode,
		ErrorMessage:        err.Error(),
	}
	if dbErr := r.repo.CreateStepExecution(step); dbErr != nil {
		slog.Warn("failed to record step failure", "stage", stage.Name, "err", dbErr)
	}
	return fmt.Errorf("stage %q: %s: %w", stage.Name, errorCode, err)
}
