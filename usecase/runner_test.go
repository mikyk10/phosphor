package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mikyk10/phosphor/config"
	"github.com/mikyk10/phosphor/llm"
	"github.com/mikyk10/phosphor/pipeline"
	"github.com/mikyk10/phosphor/store"
	"github.com/stretchr/testify/require"
)

func TestRunPipeline_ChainsStageOutputIntoNextPrompt(t *testing.T) {
	dir := t.TempDir()
	stage1 := writePrompt(t, dir, "one.md", `---
provider: test
model: m1
---
Describe the weather.`)
	stage2 := writePrompt(t, dir, "two.md", `---
provider: test
model: m2
---
Given: {{.prev.output}}
Size is {{.config.Size}}.`)

	repo := &fakeRepo{}
	factory := &stubFactory{execs: []*fakeExecutor{
		{script: []scriptedResult{textResult("sunny and 24C")}},
		{script: []scriptedResult{textResult("wear a t-shirt")}},
	}}
	runner := newTestRunner(t, repo, factory, 3)

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		Stages: []config.StageConfig{
			{Name: "weather", Output: "text", Prompt: stage1},
			{Name: "advice", Output: "text", Prompt: stage2},
		},
		ConfigVars: map[string]any{"Size": "800x480"},
	})
	require.NoError(t, err)
	require.Len(t, result.Stages, 2)
	require.Equal(t, "wear a t-shirt", result.LastTextOutput())

	// Stage 2's prompt must carry stage 1's output.
	second := factory.execs[1].calls()
	require.Len(t, second, 1)
	require.Contains(t, second[0], "Given: sunny and 24C")
	require.Contains(t, second[0], "Size is 800x480.")

	// Stage 1 saw its own prompt untouched.
	require.Equal(t, []string{"Describe the weather."}, factory.execs[0].calls())

	// Frontmatter reached the factory.
	require.Equal(t, "m1", factory.calls[0].meta.Model)
	require.Equal(t, "m2", factory.calls[1].meta.Model)

	// Both steps recorded as successful, with their outputs.
	steps := repo.steps()
	require.Len(t, steps, 2)
	for _, s := range steps {
		require.Equal(t, store.StatusSuccess, s.Status)
		require.True(t, s.FinishedAt.Valid)
	}
	outputs := repo.outputs()
	require.Len(t, outputs, 2)
	require.Equal(t, "sunny and 24C", *outputs[0].ContentText)
}

func TestRunPipeline_HistoryLimitExposesPastOutputsToPrompt(t *testing.T) {
	dir := t.TempDir()
	stage := writePrompt(t, dir, "quote.md", `---
provider: test
model: m1
---
Avoid repeating any of:
{{range .history}}- {{.}}
{{end}}`)

	repo := &fakeRepo{history: map[string][]string{
		"proverb-text": {"newest quote", "older quote"},
	}}
	factory := &stubFactory{execs: []*fakeExecutor{
		{script: []scriptedResult{textResult("a fresh quote")}},
	}}
	runner := newTestRunner(t, repo, factory, 3)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		PipelineName:   "proverbs",
		Stages: []config.StageConfig{
			{Name: "proverb-text", Output: "text", Prompt: stage, HistoryLimit: 2},
		},
	})
	require.NoError(t, err)

	calls := factory.execs[0].calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "- newest quote")
	require.Contains(t, calls[0], "- older quote")
}

func TestRunPipeline_ZeroHistoryLimitSkipsLookup(t *testing.T) {
	dir := t.TempDir()
	stage := writePrompt(t, dir, "quote.md", `---
provider: test
model: m1
---
History count: {{len .history}}`)

	// No "history" key seeded — a lookup would return nothing anyway, but a
	// non-nil map with content proves whether RecentStepOutputs was called.
	repo := &fakeRepo{history: map[string][]string{"proverb-text": {"should not appear"}}}
	factory := &stubFactory{execs: []*fakeExecutor{
		{script: []scriptedResult{textResult("a fresh quote")}},
	}}
	runner := newTestRunner(t, repo, factory, 3)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 1,
		PipelineName:   "proverbs",
		Stages: []config.StageConfig{
			{Name: "proverb-text", Output: "text", Prompt: stage}, // HistoryLimit unset
		},
	})
	require.NoError(t, err)

	require.Contains(t, factory.execs[0].calls()[0], "History count: 0")
}

func TestRunPipeline_NamedStageReference(t *testing.T) {
	dir := t.TempDir()
	first := writePrompt(t, dir, "first.md", "generate a noun")
	second := writePrompt(t, dir, "second.md", "noun was {{.stages.first.output}}")

	repo := &fakeRepo{}
	factory := &stubFactory{execs: []*fakeExecutor{
		{script: []scriptedResult{textResult("otter")}},
		{script: []scriptedResult{textResult("done")}},
	}}
	runner := newTestRunner(t, repo, factory, 3)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{
			{Name: "first", Output: "text", Prompt: first},
			{Name: "second", Output: "text", Prompt: second},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"noun was otter"}, factory.execs[1].calls())
}

func TestRunPipeline_RetriesUntilSuccess(t *testing.T) {
	dir := t.TempDir()
	prompt := writePrompt(t, dir, "p.md", "go")

	boom := errors.New("upstream hiccup")
	repo := &fakeRepo{}
	exec := &fakeExecutor{script: []scriptedResult{
		failure(boom),
		failure(boom),
		textResult("third time lucky"),
	}}
	factory := &stubFactory{execs: []*fakeExecutor{exec}}
	runner := newTestRunner(t, repo, factory, 3)

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{{Name: "s", Output: "text", Prompt: prompt}},
	})
	require.NoError(t, err)
	require.Equal(t, "third time lucky", result.LastTextOutput())
	require.Len(t, exec.calls(), 3, "two failures should have been retried")

	// One executor is built for the stage, not one per attempt.
	require.Len(t, factory.calls, 1)

	steps := repo.steps()
	require.Len(t, steps, 1)
	require.Equal(t, store.StatusSuccess, steps[0].Status)
}

func TestRunPipeline_ExhaustsRetriesAndRecordsFailure(t *testing.T) {
	dir := t.TempDir()
	prompt := writePrompt(t, dir, "p.md", "go")

	boom := errors.New("provider is down")
	repo := &fakeRepo{}
	exec := &fakeExecutor{script: []scriptedResult{failure(boom)}}
	factory := &stubFactory{execs: []*fakeExecutor{exec}}
	runner := newTestRunner(t, repo, factory, 2)

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		PipelineExecID: 7,
		Stages:         []config.StageConfig{{Name: "doomed", Output: "text", Prompt: prompt}},
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.ErrorIs(t, err, boom)
	require.Contains(t, err.Error(), `stage "doomed" failed`)

	// maxRetries=2 means attempts 0, 1 and 2.
	require.Len(t, exec.calls(), 3)

	steps := repo.steps()
	require.Len(t, steps, 1)
	require.Equal(t, store.StatusFailed, steps[0].Status)
	require.Equal(t, "execution_failed", steps[0].ErrorCode)
	require.Equal(t, boom.Error(), steps[0].ErrorMessage)
	require.EqualValues(t, 7, steps[0].PipelineExecutionID)
	require.True(t, steps[0].FinishedAt.Valid)

	// The failure was written back, not just mutated in memory.
	require.NotEmpty(t, repo.stepUpdates)
	last := repo.stepUpdates[len(repo.stepUpdates)-1]
	require.Equal(t, store.StatusFailed, last.Status)

	// A failed stage stores no output.
	require.Empty(t, repo.outputs())
}

func TestRunPipeline_StopsAtNonRetryableError(t *testing.T) {
	dir := t.TempDir()
	prompt := writePrompt(t, dir, "p.md", "go")

	rejected := fmt.Errorf("image edit %w", &llm.StatusError{StatusCode: 400, Body: "policy violation"})
	repo := &fakeRepo{}
	exec := &fakeExecutor{script: []scriptedResult{failure(rejected)}}
	factory := &stubFactory{execs: []*fakeExecutor{exec}}
	runner := newTestRunner(t, repo, factory, 3)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{{Name: "gen", Output: "image", Prompt: prompt}},
	})
	require.Error(t, err)
	require.Len(t, exec.calls(), 1, "a 400 must not be paid for again")
	require.Equal(t, store.StatusFailed, repo.steps()[0].Status)
}

func TestRunPipeline_PassesSourceImageToStage(t *testing.T) {
	dir := t.TempDir()
	prompt := writePrompt(t, dir, "p.md", "stylize")

	source := []byte{0xFF, 0xD8, 0xFF}
	repo := &fakeRepo{}
	exec := &fakeExecutor{script: []scriptedResult{imageResult([]byte("png-bytes"))}}
	factory := &stubFactory{execs: []*fakeExecutor{exec}}
	runner := newTestRunner(t, repo, factory, 1)

	result, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages:      []config.StageConfig{{Name: "stylize", Output: "image", Prompt: prompt, ImageInput: "_source"}},
		SourceImage: source,
		ExecutionParams: pipeline.ExecutionParams{
			Size:    "1024x1024",
			Quality: "low",
		},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{source}, exec.imagesFor(0))

	img, ct := result.LastImageOutput()
	require.Equal(t, []byte("png-bytes"), img)
	require.Equal(t, "image/png", ct)

	// Runtime overrides reach the factory.
	require.Equal(t, "1024x1024", factory.calls[0].params.Size)
	require.Equal(t, "low", factory.calls[0].params.Quality)
	require.Equal(t, "image", factory.calls[0].outputType)
}

func TestRunPipeline_MissingSourceImageIsRecordedAsFailure(t *testing.T) {
	dir := t.TempDir()
	prompt := writePrompt(t, dir, "p.md", "stylize")

	repo := &fakeRepo{}
	factory := &stubFactory{execs: []*fakeExecutor{{script: []scriptedResult{textResult("never runs")}}}}
	runner := newTestRunner(t, repo, factory, 1)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{{Name: "stylize", Output: "image", Prompt: prompt, ImageInput: "_source"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "image_input_failed")

	steps := repo.steps()
	require.Len(t, steps, 1)
	require.Equal(t, store.StatusFailed, steps[0].Status)
	require.Equal(t, "image_input_failed", steps[0].ErrorCode)
	require.Empty(t, factory.calls, "no executor should have been built")
}

func TestRunPipeline_MissingPromptFile(t *testing.T) {
	repo := &fakeRepo{}
	factory := &stubFactory{}
	runner := newTestRunner(t, repo, factory, 1)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{{Name: "s", Output: "text", Prompt: t.TempDir() + "/nope.md"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "prompt_load_failed")
	require.Equal(t, "prompt_load_failed", repo.steps()[0].ErrorCode)
}

func TestRunPipeline_EmptyPromptPath(t *testing.T) {
	repo := &fakeRepo{}
	runner := newTestRunner(t, repo, &stubFactory{}, 1)

	_, err := runner.RunPipeline(context.Background(), RunPipelineInput{
		Stages: []config.StageConfig{{Name: "s", Output: "text"}},
	})
	require.Error(t, err)
	require.Equal(t, "prompt_load_failed", repo.steps()[0].ErrorCode)
}

func TestRunPipeline_CancelledContextStopsBeforeFirstStage(t *testing.T) {
	dir := t.TempDir()
	prompt := writePrompt(t, dir, "p.md", "go")

	repo := &fakeRepo{}
	exec := &fakeExecutor{script: []scriptedResult{textResult("unreachable")}}
	factory := &stubFactory{execs: []*fakeExecutor{exec}}
	runner := newTestRunner(t, repo, factory, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runner.RunPipeline(ctx, RunPipelineInput{
		Stages: []config.StageConfig{{Name: "s", Output: "text", Prompt: prompt}},
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, exec.calls())
}

func TestResolveImageInput(t *testing.T) {
	source := []byte("source-bytes")
	stageOutputs := map[string]llm.StageOutput{
		"picture": {Image: []byte("stage-bytes")},
		"caption": {Text: "just text"},
	}

	t.Run("_source returns the request body", func(t *testing.T) {
		got, err := resolveImageInput("_source", source, stageOutputs)
		require.NoError(t, err)
		require.Equal(t, source, got)
	})

	t.Run("_source without a body is an error", func(t *testing.T) {
		_, err := resolveImageInput("_source", nil, stageOutputs)
		require.ErrorContains(t, err, "_source referenced but no source image provided")
	})

	t.Run("named stage returns its image", func(t *testing.T) {
		got, err := resolveImageInput("picture", source, stageOutputs)
		require.NoError(t, err)
		require.Equal(t, []byte("stage-bytes"), got)
	})

	t.Run("unknown stage is an error", func(t *testing.T) {
		_, err := resolveImageInput("ghost", source, stageOutputs)
		require.ErrorContains(t, err, `image_input references unknown stage "ghost"`)
	})

	t.Run("stage without an image is an error", func(t *testing.T) {
		_, err := resolveImageInput("caption", source, stageOutputs)
		require.ErrorContains(t, err, `stage "caption" has no image output`)
	})
}
