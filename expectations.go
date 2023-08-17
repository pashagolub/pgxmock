package pgxmock

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
)

// an expectation interface
type expectation interface {
	fulfilled() bool
	Lock()
	Unlock()
	String() string
}

// common expectation struct
// satisfies the expectation interface
type commonExpectation struct {
	sync.Mutex
	triggered bool
	err       error
}

func (e *commonExpectation) fulfilled() bool {
	return e.triggered
}

// ExpectedClose is used to manage pgx.Close expectation
// returned by pgxmock.ExpectClose.
type ExpectedClose struct {
	commonExpectation
}

// WillReturnError allows to set an error for pgx.Close action
func (e *ExpectedClose) WillReturnError(err error) *ExpectedClose {
	e.err = err
	return e
}

// String returns string representation
func (e *ExpectedClose) String() string {
	msg := "ExpectedClose => expecting database Close"
	if e.err != nil {
		msg += fmt.Sprintf(", which should return error: %s", e.err)
	}
	return msg
}

// ExpectedBegin is used to manage *pgx.Begin expectation
// returned by pgxmock.ExpectBegin.
type ExpectedBegin struct {
	commonExpectation
	delay time.Duration
	opts  pgx.TxOptions
}

// WillReturnError allows to set an error for pgx.Begin action
func (e *ExpectedBegin) WillReturnError(err error) *ExpectedBegin {
	e.err = err
	return e
}

// String returns string representation
func (e *ExpectedBegin) String() string {
	msg := "ExpectedBegin => expecting database transaction Begin"
	if e.err != nil {
		msg += fmt.Sprintf(", which should return error: %s", e.err)
	}
	return msg
}

// WillDelayFor allows to specify duration for which it will delay
// result. May be used together with Context
func (e *ExpectedBegin) WillDelayFor(duration time.Duration) *ExpectedBegin {
	e.delay = duration
	return e
}

// ExpectedCommit is used to manage pgx.Tx.Commit expectation
// returned by pgxmock.ExpectCommit.
type ExpectedCommit struct {
	commonExpectation
}

// WillReturnError allows to set an error for pgx.Tx.Close action
func (e *ExpectedCommit) WillReturnError(err error) *ExpectedCommit {
	e.err = err
	return e
}

// String returns string representation
func (e *ExpectedCommit) String() string {
	msg := "ExpectedCommit => expecting transaction Commit"
	if e.err != nil {
		msg += fmt.Sprintf(", which should return error: %s", e.err)
	}
	return msg
}

// ExpectedRollback is used to manage pgx.Tx.Rollback expectation
// returned by pgxmock.ExpectRollback.
type ExpectedRollback struct {
	commonExpectation
}

// WillReturnError allows to set an error for pgx.Tx.Rollback action
func (e *ExpectedRollback) WillReturnError(err error) *ExpectedRollback {
	e.err = err
	return e
}

// String returns string representation
func (e *ExpectedRollback) String() string {
	msg := "ExpectedRollback => expecting transaction Rollback"
	if e.err != nil {
		msg += fmt.Sprintf(", which should return error: %s", e.err)
	}
	return msg
}

// ExpectedQuery is used to manage *pgx.Conn.Query, *pgx.Conn.QueryRow, *pgx.Tx.Query,
// *pgx.Tx.QueryRow, *pgx.Stmt.Query or *pgx.Stmt.QueryRow expectations.
// Returned by pgxmock.ExpectQuery.
type ExpectedQuery struct {
	queryBasedExpectation
	rows             pgx.Rows
	delay            time.Duration
	rowsMustBeClosed bool
	rowsWereClosed   bool
}

// WithArgs will match given expected args to actual database query arguments.
// if at least one argument does not match, it will return an error. For specific
// arguments an pgxmock.Argument interface can be used to match an argument.
func (e *ExpectedQuery) WithArgs(args ...interface{}) *ExpectedQuery {
	e.args = args
	return e
}

// RowsWillBeClosed expects this query rows to be closed.
func (e *ExpectedQuery) RowsWillBeClosed() *ExpectedQuery {
	e.rowsMustBeClosed = true
	return e
}

// WillReturnError allows to set an error for expected database query
func (e *ExpectedQuery) WillReturnError(err error) *ExpectedQuery {
	e.err = err
	return e
}

// WillDelayFor allows to specify duration for which it will delay
// result. May be used together with Context
func (e *ExpectedQuery) WillDelayFor(duration time.Duration) *ExpectedQuery {
	e.delay = duration
	return e
}

// String returns string representation
func (e *ExpectedQuery) String() string {
	msg := "ExpectedQuery => expecting Query, QueryContext or QueryRow which:"
	msg += "\n  - matches sql: '" + e.expectSQL + "'"

	if len(e.args) == 0 {
		msg += "\n  - is without arguments"
	} else {
		msg += "\n  - is with arguments:\n"
		for i, arg := range e.args {
			msg += fmt.Sprintf("    %d - %+v\n", i, arg)
		}
		msg = strings.TrimSpace(msg)
	}

	if e.rows != nil {
		msg += fmt.Sprintf("\n  - %s", e.rows)
	}

	if e.err != nil {
		msg += fmt.Sprintf("\n  - should return error: %s", e.err)
	}

	return msg
}

// ExpectedExec is used to manage pgx.Exec, pgx.Tx.Exec or pgx.Stmt.Exec expectations.
// Returned by pgxmock.ExpectExec.
type ExpectedExec struct {
	queryBasedExpectation
	result pgconn.CommandTag
	delay  time.Duration
}

// WithArgs will match given expected args to actual database exec operation arguments.
// if at least one argument does not match, it will return an error. For specific
// arguments an pgxmock.Argument interface can be used to match an argument.
func (e *ExpectedExec) WithArgs(args ...interface{}) *ExpectedExec {
	e.args = args
	return e
}

// WillReturnError allows to set an error for expected database exec action
func (e *ExpectedExec) WillReturnError(err error) *ExpectedExec {
	e.err = err
	return e
}

// WillDelayFor allows to specify duration for which it will delay
// result. May be used together with Context
func (e *ExpectedExec) WillDelayFor(duration time.Duration) *ExpectedExec {
	e.delay = duration
	return e
}

// String returns string representation
func (e *ExpectedExec) String() string {
	msg := "ExpectedExec => expecting Exec or ExecContext which:"
	msg += "\n  - matches sql: '" + e.expectSQL + "'"

	if len(e.args) == 0 {
		msg += "\n  - is without arguments"
	} else {
		msg += "\n  - is with arguments:\n"
		var margs []string
		for i, arg := range e.args {
			margs = append(margs, fmt.Sprintf("    %d - %+v", i, arg))
		}
		msg += strings.Join(margs, "\n")
	}

	if e.result.String() > "" {
		msg += "\n  - should return Result having:"
		msg += fmt.Sprintf("\n      RowsAffected: %d", e.result.RowsAffected())
	}

	if e.err != nil {
		msg += fmt.Sprintf("\n  - should return error: %s", e.err)
	}

	return msg
}

// WillReturnResult arranges for an expected Exec() to return a particular
// result, there is pgxmock.NewResult(lastInsertID int64, affectedRows int64) method
// to build a corresponding result. Or if actions needs to be tested against errors
// pgxmock.NewErrorResult(err error) to return a given error.
func (e *ExpectedExec) WillReturnResult(result pgconn.CommandTag) *ExpectedExec {
	e.result = result
	return e
}

// ExpectedPrepare is used to manage pgx.Prepare or pgx.Tx.Prepare expectations.
// Returned by pgxmock.ExpectPrepare.
type ExpectedPrepare struct {
	commonExpectation
	mock           *pgxmock
	expectStmtName string
	expectSQL      string
	closeErr       error
	mustBeClosed   bool
	wasClosed      bool
	delay          time.Duration
}

// WillReturnError allows to set an error for the expected pgx.Prepare or pgx.Tx.Prepare action.
func (e *ExpectedPrepare) WillReturnError(err error) *ExpectedPrepare {
	e.err = err
	return e
}

// WillReturnCloseError allows to set an error for this prepared statement Close action
func (e *ExpectedPrepare) WillReturnCloseError(err error) *ExpectedPrepare {
	e.closeErr = err
	return e
}

// WillDelayFor allows to specify duration for which it will delay
// result. May be used together with Context
func (e *ExpectedPrepare) WillDelayFor(duration time.Duration) *ExpectedPrepare {
	e.delay = duration
	return e
}

// WillBeClosed expects this prepared statement to
// be closed.
func (e *ExpectedPrepare) WillBeClosed() *ExpectedPrepare {
	e.mustBeClosed = true
	return e
}

// ExpectQuery allows to expect Query() or QueryRow() on this prepared statement.
// This method is convenient in order to prevent duplicating sql query string matching.
func (e *ExpectedPrepare) ExpectQuery() *ExpectedQuery {
	eq := &ExpectedQuery{}
	eq.expectSQL = e.expectStmtName
	// eq.converter = e.mock.converter
	e.mock.expected = append(e.mock.expected, eq)
	return eq
}

// ExpectExec allows to expect Exec() on this prepared statement.
// This method is convenient in order to prevent duplicating sql query string matching.
func (e *ExpectedPrepare) ExpectExec() *ExpectedExec {
	eq := &ExpectedExec{}
	eq.expectSQL = e.expectStmtName
	// eq.converter = e.mock.converter
	e.mock.expected = append(e.mock.expected, eq)
	return eq
}

// String returns string representation
func (e *ExpectedPrepare) String() string {
	msg := "ExpectedPrepare => expecting Prepare statement which:"
	msg += "\n  - matches statement name: '" + e.expectStmtName + "'"
	msg += "\n  - matches sql: '" + e.expectSQL + "'"

	if e.err != nil {
		msg += fmt.Sprintf("\n  - should return error: %s", e.err)
	}

	if e.closeErr != nil {
		msg += fmt.Sprintf("\n  - should return error on Close: %s", e.closeErr)
	}

	return msg
}

// query based expectation
// adds a query matching logic
type queryBasedExpectation struct {
	commonExpectation
	expectSQL string
	// converter driver.ValueConverter
	args []interface{}
}

// ExpectedPing is used to manage pgx.Ping expectations.
// Returned by pgxmock.ExpectPing.
type ExpectedPing struct {
	commonExpectation
	delay time.Duration
}

// WillDelayFor allows to specify duration for which it will delay result. May
// be used together with Context.
func (e *ExpectedPing) WillDelayFor(duration time.Duration) *ExpectedPing {
	e.delay = duration
	return e
}

// WillReturnError allows to set an error for expected database ping
func (e *ExpectedPing) WillReturnError(err error) *ExpectedPing {
	e.err = err
	return e
}

// String returns string representation
func (e *ExpectedPing) String() string {
	msg := "ExpectedPing => expecting database Ping"
	if e.err != nil {
		msg += fmt.Sprintf(", which should return error: %s", e.err)
	}
	return msg
}

// WillReturnRows specifies the set of resulting rows that will be returned
// by the triggered query
func (e *ExpectedQuery) WillReturnRows(rows ...*Rows) *ExpectedQuery {
	e.rows = &rowSets{sets: rows, ex: e}
	return e
}

func (e *queryBasedExpectation) argsMatches(args []interface{}) error {
	if len(args) != len(e.args) {
		return fmt.Errorf("expected %d, but got %d arguments", len(e.args), len(args))
	}
	for k, v := range args {
		// custom argument matcher
		matcher, ok := e.args[k].(Argument)
		if ok {
			if !matcher.Match(v) {
				return fmt.Errorf("matcher %T could not match %d argument %T - %+v", matcher, k, args[k], args[k])
			}
			continue
		}
		darg := e.args[k]
		if !reflect.DeepEqual(darg, v) {
			return fmt.Errorf("argument %d expected [%T - %+v] does not match actual [%T - %+v]", k, darg, darg, v, v)
		}
	}
	return nil
}

func (e *queryBasedExpectation) attemptArgMatch(args []interface{}) (err error) {
	// catch panic
	defer func() {
		if e := recover(); e != nil {
			_, ok := e.(error)
			if !ok {
				err = fmt.Errorf(e.(string))
			}
		}
	}()

	err = e.argsMatches(args)
	return
}

// ExpectedCopyFrom is used to manage *pgx.Conn.CopyFrom expectations.
// Returned by *Pgxmock.ExpectCopyFrom.
type ExpectedCopyFrom struct {
	commonExpectation
	expectedTableName pgx.Identifier
	expectedColumns   []string
	rowsAffected      int64
	delay             time.Duration
}

// WillReturnError allows to set an error for expected database exec action
func (e *ExpectedCopyFrom) WillReturnError(err error) *ExpectedCopyFrom {
	e.err = err
	return e
}

// WillDelayFor allows to specify duration for which it will delay
// result. May be used together with Context
func (e *ExpectedCopyFrom) WillDelayFor(duration time.Duration) *ExpectedCopyFrom {
	e.delay = duration
	return e
}

// String returns string representation
func (e *ExpectedCopyFrom) String() string {
	msg := "ExpectedCopyFrom => expecting CopyFrom which:"
	msg += "\n  - matches table name: '" + e.expectedTableName.Sanitize() + "'"
	msg += fmt.Sprintf("\n  - matches column names: '%+v'", e.expectedColumns)

	if e.err != nil {
		msg += fmt.Sprintf("\n  - should return error: %s", e.err)
	}

	return msg
}

// WillReturnResult arranges for an expected Exec() to return a particular
// result, there is pgxmock.NewResult(lastInsertID int64, affectedRows int64) method
// to build a corresponding result. Or if actions needs to be tested against errors
// pgxmock.NewErrorResult(err error) to return a given error.
func (e *ExpectedCopyFrom) WillReturnResult(result int64) *ExpectedCopyFrom {
	e.rowsAffected = result
	return e
}

// ExpectedReset is used to manage pgx.Reset expectation
type ExpectedReset struct {
	commonExpectation
}

func (e *ExpectedReset) String() string {
	return "ExpectedReset => expecting database Reset"
}
