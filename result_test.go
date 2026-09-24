package pgxmock

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldReturnValidSqlDriverResult(t *testing.T) {
	result := NewResult("SELECT", 2)
	if !result.Select() {
		t.Errorf("expected SELECT operation result, but got: %v", result.String())
	}
	affected := result.RowsAffected()
	if affected != 2 {
		t.Errorf("expected affected rows to be 2, but got: %d", affected)
	}
}

func TestExecReturnsNoTagOnError(t *testing.T) {
	errExec := errors.New("exec")
	mock, _ := NewConn()
	mock.ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1)).WillReturnError(errExec)

	tag, err := mock.Exec(ctx, "UPDATE")
	assert.ErrorIs(t, err, errExec)
	assert.Zero(t, tag.RowsAffected())
}
