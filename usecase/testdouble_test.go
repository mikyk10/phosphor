package usecase

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mikyk10/phosphor/config"
	"github.com/mikyk10/phosphor/llm"
	"github.com/mikyk10/phosphor/pipeline"
	"github.com/mikyk10/phosphor/store"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory repository
// ---------------------------------------------------------------------------

// fakeRepo records everything the runner persists. store.Repository is
// embedded (and left nil) so that adding a method to the interface does not
// break this double; any unimplemented call panics loudly instead.
type fakeRepo struct {
	store.Repository

	mu sync.Mutex

	nextID       store.PrimaryKey
	pipelineRows []*store.PipelineExecution
	stepRows     []*store.StepExecution
	outputRows   []*store.StepOutput

	// stepUpdates snapshots each UpdateStepExecution call, so a test can see
	// the state that was actually written rather than the live pointer.
	stepUpdates []store.StepExecution

	// history is returned by RecentStepOutputs, keyed by stage name.
	history map[string][]string
}

func (r *fakeRepo) CreatePipelineExecution(exec *store.PipelineExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	exec.ID = r.nextID
	r.pipelineRows = append(r.pipelineRows, exec)
	return nil
}

func (r *fakeRepo) UpdatePipelineExecution(*store.PipelineExecution) error { return nil }

func (r *fakeRepo) CreateStepExecution(step *store.StepExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	step.ID = r.nextID
	r.stepRows = append(r.stepRows, step)
	return nil
}

func (r *fakeRepo) UpdateStepExecution(step *store.StepExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepUpdates = append(r.stepUpdates, *step)
	return nil
}

func (r *fakeRepo) CreateStepOutput(out *store.StepOutput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outputRows = append(r.outputRows, out)
	return nil
}

func (r *fakeRepo) RecentStepOutputs(_, stageName string, limit int) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	results := r.history[stageName]
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	return results, nil
}

func (r *fakeRepo) steps() []*store.StepExecution {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*store.StepExecution(nil), r.stepRows...)
}

func (r *fakeRepo) outputs() []*store.StepOutput {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*store.StepOutput(nil), r.outputRows...)
}

// ---------------------------------------------------------------------------
// Scripted stage executor
// ---------------------------------------------------------------------------

// scriptedResult is one scripted Execute outcome.
type scriptedResult struct {
	result *pipeline.StageResult
	err    error
}

func textResult(s string) scriptedResult {
	return scriptedResult{result: &pipeline.StageResult{OutputType: "text", Text: s}}
}

func imageResult(data []byte) scriptedResult {
	return scriptedResult{result: &pipeline.StageResult{
		OutputType:  "image",
		ImageData:   data,
		ContentType: "image/png",
	}}
}

func failure(err error) scriptedResult { return scriptedResult{err: err} }

// fakeExecutor returns scripted results and records how it was called. The
// last scripted entry is repeated once the script runs out.
type fakeExecutor struct {
	mu      sync.Mutex
	script  []scriptedResult
	prompts []string
	images  [][][]byte
}

func (e *fakeExecutor) Execute(_ context.Context, prompt string, images [][]byte) (*pipeline.StageResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	idx := len(e.prompts)
	e.prompts = append(e.prompts, prompt)
	e.images = append(e.images, images)

	if idx >= len(e.script) {
		idx = len(e.script) - 1
	}
	s := e.script[idx]
	if s.err != nil {
		return nil, s.err
	}
	// Copy so the runner's mutation of StageName does not leak between calls.
	res := *s.result
	return &res, nil
}

func (e *fakeExecutor) calls() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.prompts...)
}

func (e *fakeExecutor) imagesFor(call int) [][]byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.images[call]
}

// factoryCall records the arguments the runner passed to the factory.
type factoryCall struct {
	meta       llm.PromptMeta
	outputType string
	timeout    time.Duration
	params     pipeline.ExecutionParams
}

// stubFactory hands out one fakeExecutor per stage, in stage order.
type stubFactory struct {
	mu    sync.Mutex
	execs []*fakeExecutor
	calls []factoryCall
	err   error
}

func (f *stubFactory) build(_ map[string]config.ProviderConfig, meta llm.PromptMeta, outputType string, timeout time.Duration, params pipeline.ExecutionParams) (pipeline.StageExecutor, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return nil, f.err
	}
	idx := len(f.calls)
	f.calls = append(f.calls, factoryCall{meta: meta, outputType: outputType, timeout: timeout, params: params})
	if idx >= len(f.execs) {
		return nil, os.ErrNotExist
	}
	return f.execs[idx], nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestRunner builds a runner wired to the doubles, with a retry backoff
// short enough that a full retry sequence stays sub-millisecond.
func newTestRunner(t *testing.T, repo store.Repository, factory *stubFactory, maxRetries int) *PipelineRunner {
	t.Helper()
	return &PipelineRunner{
		cfg: &config.GlobalConfig{
			AI: config.AIConfig{
				RequestTimeoutSec: 30,
				MaxRetries:        maxRetries,
				Providers:         map[string]config.ProviderConfig{"test": {Endpoint: "http://localhost"}},
			},
		},
		repo:            repo,
		executorFactory: factory.build,
		backoffBase:     time.Microsecond,
	}
}

// writePrompt writes a prompt file into dir and returns its path.
func writePrompt(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}
