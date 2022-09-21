package pgxmock

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// NewResult creates a new sql driver Result
// for Exec based query mocks.
func NewResult(op string, rowsAffected int64) pgconn.CommandTag {
	return pgconn.NewCommandTag(fmt.Sprint(op, rowsAffected))
}

// NewErrorResult creates a new sql driver Result
// which returns an error given for both interface methods
func NewErrorResult(err error) pgconn.CommandTag {
	return pgconn.NewCommandTag(err.Error())
}
