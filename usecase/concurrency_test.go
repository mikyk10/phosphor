package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAcquire_Unlimited(t *testing.T) {
	u := NewPipelineUsecase(nil, nil, nil, 0).(*pipelineUsecase)
	require.Nil(t, u.sem)

	for i := 0; i < 100; i++ {
		release, err := u.acquire(context.Background())
		require.NoError(t, err)
		defer release()
	}
}

func TestAcquire_BlocksAtLimitAndResumesOnRelease(t *testing.T) {
	u := NewPipelineUsecase(nil, nil, nil, 2).(*pipelineUsecase)

	first, err := u.acquire(context.Background())
	require.NoError(t, err)
	second, err := u.acquire(context.Background())
	require.NoError(t, err)

	// The third caller must wait.
	acquired := make(chan struct{})
	go func() {
		release, err := u.acquire(context.Background())
		if err == nil {
			release()
		}
		close(acquired)
	}()

	select {
	case <-acquired:
		t.Fatal("third execution started while two slots were held")
	case <-time.After(50 * time.Millisecond):
	}

	first()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("releasing a slot did not unblock the waiter")
	}
	second()
}

func TestAcquire_GivesUpWhenCallerDisconnects(t *testing.T) {
	u := NewPipelineUsecase(nil, nil, nil, 1).(*pipelineUsecase)

	release, err := u.acquire(context.Background())
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = u.acquire(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), time.Second, "must not queue indefinitely")
}
