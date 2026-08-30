package pgxmock

import "github.com/jackc/pgx/v5/pgconn"

// NewPgError builds a *pgconn.PgError carrying the given SQLSTATE code and
// message, so that an expectation can simulate a server-side failure the way
// the code under test will really see it:
//
//	mock.ExpectExec("INSERT INTO users").
//		WillReturnError(pgxmock.NewPgError("23505", `duplicate key value violates unique constraint "users_email_key"`))
//
// The code under test can then take the branch it takes in production:
//
//	var pgErr *pgconn.PgError
//	if errors.As(err, &pgErr) && pgErr.Code == "23505" { ... }
//
// Severity is filled in as "ERROR". Set any of the remaining fields, such as
// ConstraintName or TableName, on the returned value.
func NewPgError(code, message string) *pgconn.PgError {
	return &pgconn.PgError{
		Severity:            "ERROR",
		SeverityUnlocalized: "ERROR",
		Code:                code,
		Message:             message,
	}
}
