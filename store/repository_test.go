package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := NewSQLiteConnection(":memory:", true)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(AllModels()...))
	return db
}

// seedExecutions creates n pipeline executions, each with one step and one
// step output, and returns their IDs in creation order.
func seedExecutions(t *testing.T, repo Repository, n int) []PrimaryKey {
	t.Helper()
	ids := make([]PrimaryKey, 0, n)
	for i := 0; i < n; i++ {
		exec := &PipelineExecution{
			PipelineName: "test",
			Status:       StatusSuccess,
			StartedAt:    time.Now(),
			FinishedAt:   sql.NullTime{Time: time.Now(), Valid: true},
		}
		require.NoError(t, repo.CreatePipelineExecution(exec))

		step := &StepExecution{
			PipelineExecutionID: exec.ID,
			StageName:           "stage",
			StageIndex:          0,
			Status:              StatusSuccess,
			StartedAt:           time.Now(),
		}
		require.NoError(t, repo.CreateStepExecution(step))

		text := "output"
		require.NoError(t, repo.CreateStepOutput(&StepOutput{
			StepExecutionID: step.ID,
			ContentType:     "text/plain",
			ContentText:     &text,
		}))

		ids = append(ids, exec.ID)
	}
	return ids
}

func counts(t *testing.T, db *gorm.DB) (execs, steps, outputs int64) {
	t.Helper()
	require.NoError(t, db.Model(&PipelineExecution{}).Count(&execs).Error)
	require.NoError(t, db.Model(&StepExecution{}).Count(&steps).Error)
	require.NoError(t, db.Model(&StepOutput{}).Count(&outputs).Error)
	return
}

func TestPruneExecutions_DeletesOldestAndCascades(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)
	ids := seedExecutions(t, repo, 10)

	deleted, err := repo.PruneExecutions(3)
	require.NoError(t, err)
	require.EqualValues(t, 7, deleted)

	execs, steps, outputs := counts(t, db)
	require.EqualValues(t, 3, execs, "only the newest 3 executions remain")
	require.EqualValues(t, 3, steps, "related step_executions are removed")
	require.EqualValues(t, 3, outputs, "related step_outputs are removed")

	// The survivors must be the three most recent ones.
	var remaining []PrimaryKey
	require.NoError(t, db.Model(&PipelineExecution{}).Order("id").Pluck("id", &remaining).Error)
	require.Equal(t, ids[len(ids)-3:], remaining)
}

func TestPruneExecutions_NoOpWhenBelowThreshold(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)
	seedExecutions(t, repo, 3)

	deleted, err := repo.PruneExecutions(5)
	require.NoError(t, err)
	require.EqualValues(t, 0, deleted)

	execs, steps, outputs := counts(t, db)
	require.EqualValues(t, 3, execs)
	require.EqualValues(t, 3, steps)
	require.EqualValues(t, 3, outputs)
}

func TestPruneExecutions_ExactThresholdIsNoOp(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)
	seedExecutions(t, repo, 5)

	deleted, err := repo.PruneExecutions(5)
	require.NoError(t, err)
	require.EqualValues(t, 0, deleted)

	execs, _, _ := counts(t, db)
	require.EqualValues(t, 5, execs)
}

func TestPruneExecutions_KeepAllWithNegative(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)
	seedExecutions(t, repo, 10)

	for _, keep := range []int{-1, 0} {
		deleted, err := repo.PruneExecutions(keep)
		require.NoError(t, err)
		require.EqualValues(t, 0, deleted, "keep=%d must disable pruning", keep)
	}

	execs, steps, outputs := counts(t, db)
	require.EqualValues(t, 10, execs)
	require.EqualValues(t, 10, steps)
	require.EqualValues(t, 10, outputs)
}

func TestPruneExecutions_KeepOne(t *testing.T) {
	db := newTestDB(t)
	repo := NewRepository(db)
	ids := seedExecutions(t, repo, 4)

	deleted, err := repo.PruneExecutions(1)
	require.NoError(t, err)
	require.EqualValues(t, 3, deleted)

	var remaining []PrimaryKey
	require.NoError(t, db.Model(&PipelineExecution{}).Pluck("id", &remaining).Error)
	require.Equal(t, []PrimaryKey{ids[len(ids)-1]}, remaining)
}
