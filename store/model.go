package store

import (
	"database/sql"
	"time"
)

type PrimaryKey = uint

type ExecutionStatus string

const (
	StatusPending ExecutionStatus = "pending"
	StatusRunning ExecutionStatus = "running"
	StatusSuccess ExecutionStatus = "success"
	StatusFailed  ExecutionStatus = "failed"
)

type PipelineExecution struct {
	ID           PrimaryKey      `gorm:"primaryKey;autoIncrement"`
	PipelineName string          `gorm:"type:varchar(64);not null;index"`
	Status       ExecutionStatus `gorm:"type:varchar(32);not null"`
	StartedAt    time.Time       `gorm:"not null"`
	FinishedAt   sql.NullTime
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type StepExecution struct {
	ID                  PrimaryKey      `gorm:"primaryKey;autoIncrement"`
	PipelineExecutionID PrimaryKey      `gorm:"not null;index"`
	StageName           string          `gorm:"type:varchar(64);not null"`
	StageIndex          int             `gorm:"not null"`
	ProviderName        string          `gorm:"type:varchar(64)"`
	ModelName           string          `gorm:"type:varchar(128)"`
	PromptHash          string          `gorm:"type:char(12)"`
	Status              ExecutionStatus `gorm:"type:varchar(32);not null"`
	StartedAt           time.Time       `gorm:"not null"`
	FinishedAt          sql.NullTime
	LatencyMs           int64
	ErrorCode           string `gorm:"type:varchar(64)"`
	ErrorMessage        string `gorm:"type:text"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type StepOutput struct {
	ID              PrimaryKey `gorm:"primaryKey;autoIncrement"`
	StepExecutionID PrimaryKey `gorm:"not null;uniqueIndex"`
	ContentType     string     `gorm:"type:varchar(64);not null"`
	ContentText     *string    `gorm:"type:text"`
	ContentBlob     []byte     `gorm:"type:blob"`
	CreatedAt       time.Time
}

func AllModels() []any {
	return []any{
		&PipelineExecution{},
		&StepExecution{},
		&StepOutput{},
	}
}
