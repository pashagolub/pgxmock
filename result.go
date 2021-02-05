package pgxmock

import (
	"strconv"

	"github.com/jackc/pgconn"
)

// NewResult creates a new sql driver Result
// for Exec based query mocks.
func NewResult(op string, rowsAffected int64) pgconn.CommandTag {
	return pgconn.CommandTag(op + " " + strconv.FormatInt(rowsAffected, 10))
}

// NewErrorResult creates a new sql driver Result
// which returns an error given for both interface methods
func NewErrorResult(err error) pgconn.CommandTag {
	return pgconn.CommandTag(err.Error())
}
