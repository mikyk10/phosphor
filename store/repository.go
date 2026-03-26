package store

import "gorm.io/gorm"

// Repository provides persistence for pipeline execution history.
type Repository interface {
	CreatePipelineExecution(exec *PipelineExecution) error
	UpdatePipelineExecution(exec *PipelineExecution) error
	CreateStepExecution(step *StepExecution) error
	UpdateStepExecution(step *StepExecution) error
	CreateStepOutput(out *StepOutput) error
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
