package pgxmock

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// NewResult creates a new pgconn.CommandTag result
// for Exec based query mocks.
func NewResult(op string, rowsAffected int64) pgconn.CommandTag {
	return pgconn.NewCommandTag(fmt.Sprintf("%s %d", op, rowsAffected))
}
