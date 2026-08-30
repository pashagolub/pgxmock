package pgxmock

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

// AcquireFunc used to return nil without ever running the callback, so a test
// exercising pool.AcquireFunc passed while the code under test never ran.
func TestAcquireFuncReportsInsteadOfSkippingCallback(t *testing.T) {
	mock, err := NewPool()
	assert.NoError(t, err)

	called := false
	err = mock.AcquireFunc(context.Background(), func(*pgxpool.Conn) error {
		called = true
		return nil
	})

	assert.ErrorIs(t, err, ErrAcquireNotSupported)
	assert.False(t, called, "the callback cannot run, so it must not be reported as successful")
	assert.ErrorContains(t, err, "AsConn", "the error should point at the supported alternative")
}

func TestAcquireIsNotSupported(t *testing.T) {
	mock, err := NewPool()
	assert.NoError(t, err)

	conn, err := mock.Acquire(context.Background())
	assert.Nil(t, conn)
	assert.ErrorIs(t, err, ErrAcquireNotSupported)
}

func TestAcquireAllIdleIsEmpty(t *testing.T) {
	mock, err := NewPool()
	assert.NoError(t, err)
	assert.Empty(t, mock.AcquireAllIdle(context.Background()))
}
