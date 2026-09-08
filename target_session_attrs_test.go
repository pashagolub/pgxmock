package pgxmock

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pgx v5.11 gives target_session_attrs validation its own sentinel errors, so
// code that reacts to landing on the wrong kind of server - a read replica when
// it needs to write, a primary when it wants to offload a read - can test for
// them with errors.Is. They are plain errors, so an expectation returns them
// like any other; these tests pin that down.
func TestTargetSessionAttrsErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"read only", pgconn.ErrReadOnlyConnection},
		{"read write", pgconn.ErrReadWriteConnection},
		{"primary", pgconn.ErrPrimaryConnection},
		{"standby", pgconn.ErrStandbyConnection},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock, err := NewPool()
			require.NoError(t, err)
			defer mock.Close()

			mock.ExpectPing().WillReturnError(tc.err)

			err = mock.Ping(context.Background())
			assert.ErrorIs(t, err, tc.err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// A wrapped sentinel still has to be recognisable, since that is how the code
// under test will meet it once it has added context of its own.
func TestTargetSessionAttrsErrorWrapped(t *testing.T) {
	t.Parallel()
	mock, err := NewPool()
	require.NoError(t, err)
	defer mock.Close()

	wrapped := fmt.Errorf("acquiring a writer: %w", pgconn.ErrReadOnlyConnection)
	mock.ExpectExec("INSERT INTO users").WithArgs("john").WillReturnError(wrapped)

	_, err = mock.Exec(context.Background(), "INSERT INTO users(name) VALUES ($1)", "john")
	assert.ErrorIs(t, err, pgconn.ErrReadOnlyConnection)
	assert.NoError(t, mock.ExpectationsWereMet())
}
