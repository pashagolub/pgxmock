package pgxmock

import (
	"testing"
)

func TestShouldReturnValidSqlDriverResult(t *testing.T) {
	result := NewResult("SELECT", 2)
	if !result.Select() {
		t.Errorf("expected SELECT operation result, but got: %v", result.String())
	}
	affected := result.RowsAffected()
	if 2 != affected {
		t.Errorf("expected affected rows to be 2, but got: %d", affected)
	}
}
