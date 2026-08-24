package store

import "gorm.io/gorm"

// Repository provides persistence for pipeline execution history.
type Repository interface {
	CreatePipelineExecution(exec *PipelineExecution) error
	UpdatePipelineExecution(exec *PipelineExecution) error
	CreateStepExecution(step *StepExecution) error
	UpdateStepExecution(step *StepExecution) error
	CreateStepOutput(out *StepOutput) error
	// PruneExecutions deletes all but the `keep` most recent pipeline
	// executions along with their steps and outputs. It returns the number
	// of pipeline executions deleted. keep <= 0 is a no-op.
	PruneExecutions(keep int) (int64, error)
	// RecentStepOutputs returns the text outputs of the named stage's most
	// recent successful runs within the named pipeline, newest first,
	// capped at limit. limit <= 0 returns no rows.
	RecentStepOutputs(pipelineName, stageName string, limit int) ([]string, error)
}

type repositoryImpl struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repositoryImpl{db: db}
}

func (r *repositoryImpl) CreatePipelineExecution(exec *PipelineExecution) error {
	return r.db.Create(exec).Error
}

func (r *repositoryImpl) UpdatePipelineExecution(exec *PipelineExecution) error {
	return r.db.Save(exec).Error
}

func (r *repositoryImpl) CreateStepExecution(step *StepExecution) error {
	return r.db.Create(step).Error
}

func (r *repositoryImpl) UpdateStepExecution(step *StepExecution) error {
	return r.db.Save(step).Error
}

func (r *repositoryImpl) CreateStepOutput(out *StepOutput) error {
	return r.db.Create(out).Error
}

func (r *repositoryImpl) RecentStepOutputs(pipelineName, stageName string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}

	var texts []string
	err := r.db.Raw(`
		SELECT so.content_text
		FROM step_outputs so
		JOIN step_executions se ON se.id = so.step_execution_id
		JOIN pipeline_executions pe ON pe.id = se.pipeline_execution_id
		WHERE pe.pipeline_name = ? AND se.stage_name = ? AND se.status = ?
			AND so.content_text IS NOT NULL
		ORDER BY se.id DESC
		LIMIT ?
	`, pipelineName, stageName, StatusSuccess, limit).Scan(&texts).Error
	if err != nil {
		return nil, err
	}
	return texts, nil
}

func (r *repositoryImpl) PruneExecutions(keep int) (int64, error) {
	if keep <= 0 {
		return 0, nil
	}

	var deleted int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// The ID of the oldest execution we want to keep. If fewer than
		// `keep` rows exist the query returns no row and there is nothing
		// to prune.
		var cutoff PrimaryKey
		row := tx.Raw(
			"SELECT id FROM pipeline_executions ORDER BY id DESC LIMIT 1 OFFSET ?",
			keep-1,
		).Row()
		if err := row.Scan(&cutoff); err != nil {
			// No row means the table holds fewer than `keep` executions.
			return nil //nolint:nilerr // not enough history to prune
		}

		// Delete leaves first so we never orphan rows if the tx aborts.
		if err := tx.Exec(`DELETE FROM step_outputs WHERE step_execution_id IN (
			SELECT id FROM step_executions WHERE pipeline_execution_id < ?
		)`, cutoff).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			"DELETE FROM step_executions WHERE pipeline_execution_id < ?", cutoff,
		).Error; err != nil {
			return err
		}
		res := tx.Exec("DELETE FROM pipeline_executions WHERE id < ?", cutoff)
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
