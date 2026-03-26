package pipeline

import "context"

// StageExecutor executes a single pipeline stage.
// The implementation is chosen based on the combination of output type and api_type.
type StageExecutor interface {
	Execute(ctx context.Context, prompt string, images [][]byte) (*StageResult, error)
}

// ExecutionParams holds runtime parameters that override prompt frontmatter.
// Zero values mean "use frontmatter default".
type ExecutionParams struct {
	Size    string // e.g. "1024x1024", "1536x1024"
	Quality string // e.g. "low", "high"
}

// StageResult holds the output of a single pipeline stage execution.
type StageResult struct {
	StageName   string
	OutputType  string // "text" or "image"
	Text        string
	ImageData   []byte
	ContentType string // MIME type for image, e.g. "image/png"
}

// PipelineResult holds the complete output of a pipeline execution.
type PipelineResult struct {
	Stages []StageResult
}

// LastTextOutput returns the text output of the last text-producing stage.
func (r *PipelineResult) LastTextOutput() string {
	for i := len(r.Stages) - 1; i >= 0; i-- {
		if r.Stages[i].OutputType == "text" {
			return r.Stages[i].Text
		}
	}
	return ""
}

// LastImageOutput returns the image data and content type of the last image-producing stage.
func (r *PipelineResult) LastImageOutput() ([]byte, string) {
	for i := len(r.Stages) - 1; i >= 0; i-- {
		if r.Stages[i].OutputType == "image" {
			return r.Stages[i].ImageData, r.Stages[i].ContentType
		}
	}
	return nil, ""
}

// StageOutputByName returns the output of a named stage, or nil if not found.
func (r *PipelineResult) StageOutputByName(name string) *StageResult {
	for i := range r.Stages {
		if r.Stages[i].StageName == name {
			return &r.Stages[i]
		}
	}
	return nil
}
