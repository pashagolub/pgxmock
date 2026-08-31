package pgxmock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func execWithArgs(t *testing.T, expected []any, actual []any) error {
	t.Helper()
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.ExpectExec("INSERT").WithArgs(expected...).
		WillReturnResult(NewResult("INSERT", 1))
	_, err = mock.Exec(context.Background(), "INSERT INTO t VALUES ($1)", actual...)
	return err
}

func TestNotNilMatcher(t *testing.T) {
	assert.NoError(t, execWithArgs(t, []any{NotNil()}, []any{42}))
	assert.NoError(t, execWithArgs(t, []any{NotNil()}, []any{""}))
	assert.Error(t, execWithArgs(t, []any{NotNil()}, []any{nil}))

	// a typed nil is nil too, which a bare `== nil` check would miss
	var nilPtr *int
	assert.Error(t, execWithArgs(t, []any{NotNil()}, []any{nilPtr}))
	var nilMap map[string]int
	assert.Error(t, execWithArgs(t, []any{NotNil()}, []any{nilMap}))
}

func TestAnyOfMatcher(t *testing.T) {
	assert.NoError(t, execWithArgs(t, []any{AnyOf("pending", "active")}, []any{"active"}))
	assert.Error(t, execWithArgs(t, []any{AnyOf("pending", "active")}, []any{"done"}))

	// matchers may be nested inside AnyOf
	assert.NoError(t, execWithArgs(t, []any{AnyOf("pending", NotNil())}, []any{42}))
	assert.Error(t, execWithArgs(t, []any{AnyOf("pending", NotNil())}, []any{nil}))
}

func TestOfTypeMatcher(t *testing.T) {
	assert.NoError(t, execWithArgs(t, []any{OfType[time.Time]()}, []any{time.Now()}))
	assert.Error(t, execWithArgs(t, []any{OfType[time.Time]()}, []any{"2026-08-30"}),
		"the right value in the wrong type must not match")
	assert.NoError(t, execWithArgs(t, []any{OfType[int]()}, []any{7}))
	assert.Error(t, execWithArgs(t, []any{OfType[int]()}, []any{int64(7)}))
}

func TestArgumentFunc(t *testing.T) {
	positive := ArgumentFunc(func(v any) bool {
		n, ok := v.(int)
		return ok && n > 0
	})
	assert.NoError(t, execWithArgs(t, []any{positive}, []any{7}))
	assert.Error(t, execWithArgs(t, []any{positive}, []any{-1}))
}

func TestQueryMatcherSubstring(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherSubstring))
	assert.NoError(t, err)

	// no regexp escaping needed for the parentheses or the dollar sign
	mock.ExpectQuery("WHERE (id, tenant) = ($1, $2)").
		WillReturnRows(NewRows([]string{"id"}).AddRow(1))

	rows, err := mock.Query(context.Background(),
		"SELECT id\n  FROM users\n  WHERE (id, tenant) = ($1, $2)\n  LIMIT 1")
	assert.NoError(t, err)
	rows.Close()
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryMatcherSubstringMismatch(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherSubstring))
	assert.NoError(t, err)
	mock.ExpectQuery("FROM accounts").
		WillReturnRows(NewRows([]string{"id"}).AddRow(1))

	_, err = mock.Query(context.Background(), "SELECT id FROM users")
	assert.ErrorContains(t, err, "does not contain expected")
}

// A simulated constraint violation must reach the code under test as the
// *pgconn.PgError it would really get.
func TestNewPgError(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	violation := NewPgError("23505", `duplicate key value violates unique constraint "users_email_key"`)
	violation.ConstraintName = "users_email_key"
	violation.TableName = "users"
	mock.ExpectExec("INSERT").WithArgs("a@b.c").WillReturnError(violation)

	_, err = mock.Exec(context.Background(), "INSERT INTO users (email) VALUES ($1)", "a@b.c")

	var pgErr *pgconn.PgError
	assert.True(t, errors.As(err, &pgErr), "the error must unwrap to a *pgconn.PgError")
	assert.Equal(t, "23505", pgErr.Code)
	assert.Equal(t, "23505", pgErr.SQLState())
	assert.Equal(t, "users_email_key", pgErr.ConstraintName)
	assert.Equal(t, "ERROR", pgErr.Severity)
	assert.ErrorContains(t, err, "duplicate key value")
	assert.NoError(t, mock.ExpectationsWereMet())
}
