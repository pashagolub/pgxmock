package pgxmock

import (
	"context"
	"fmt"
	"testing"
)

// used for examples
var mock = &pgxmock{}

func ExampleNewErrorResult() {
	mock, _ := New()
	result := NewErrorResult(fmt.Errorf("some error"))
	mock.ExpectExec("^INSERT (.+)").WillReturnResult(result)
	res, _ := mock.Exec(context.Background(), "INSERT something")
	s := res.String()
	fmt.Println(s)
	// Output: some error
}

func ExampleNewResult() {
	var affected int64
	result := NewResult("INSERT", affected)
	mock.ExpectExec("^INSERT (.+)").WillReturnResult(result)
	fmt.Println(mock.ExpectationsWereMet())
	// Output: there is a remaining expectation which was not matched: ExpectedExec => expecting Exec or ExecContext which:
	//   - matches sql: '^INSERT (.+)'
	//   - is without arguments
	//   - should return Result having:
	//       LastInsertId: 0
	//       RowsAffected: 0
}

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
