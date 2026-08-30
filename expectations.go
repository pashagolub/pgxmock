package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
)

// an expectation interface
type expectation interface {
	error() error
	required() bool
	fulfilled() bool
	fulfill()
	sync.Locker
	fmt.Stringer
}

// CallModifier interface represents common interface for all expectations supported
type CallModifier interface {
	// Maybe allows the expected method call to be optional.
	// Not calling an optional method will not cause an error while asserting expectations
	Maybe() CallModifier
	// Times indicates that that the expected method should only fire the indicated number of times.
	// Zero value is ignored and means the same as one.
	Times(n uint) CallModifier
	// WillDelayFor allows to specify duration for which it will delay
	// result. May be used together with Context
	WillDelayFor(duration time.Duration) CallModifier
	// WillReturnError allows to set an error for the expected method
	WillReturnError(err error)
	// WillPanic allows to force the expected method to panic
	WillPanic(v any)
}

// common expectation struct
// satisfies the expectation interface
type commonExpectation struct {
	sync.Mutex
	triggered     uint          // how many times method was called
	err           error         // should method return error
	optional      bool          // can method be skipped
	panicArgument any           // panic value to return for recovery
	plannedDelay  time.Duration // should method delay before return
	plannedCalls  uint          // how many sequentional calls should be made
}

func (e *commonExpectation) error() error {
	return e.err
}

func (e *commonExpectation) fulfill() {
	e.triggered++
}

func (e *commonExpectation) fulfilled() bool {
	return e.triggered >= max(e.plannedCalls, 1)
}

func (e *commonExpectation) required() bool {
	return !e.optional
}

func (e *commonExpectation) waitForDelay(ctx context.Context) (err error) {
	select {
	case <-time.After(e.plannedDelay):
		err = e.error()
	case <-ctx.Done():
		err = ctx.Err()
	}
	if e.panicArgument != nil {
		panic(e.panicArgument)
	}
	return err
}

func (e *commonExpectation) Maybe() CallModifier {
	e.optional = true
	return e
}

func (e *commonExpectation) Times(n uint) CallModifier {
	e.plannedCalls = n
	return e
}

func (e *commonExpectation) WillDelayFor(duration time.Duration) CallModifier {
	e.plannedDelay = duration
	return e
}

func (e *commonExpectation) WillReturnError(err error) {
	e.err = err
}

var errPanic = errors.New("pgxmock panic")

func (e *commonExpectation) WillPanic(v any) {
	e.err = errPanic
	e.panicArgument = v
}

// String returns string representation
func (e *commonExpectation) String() string {
	w := new(strings.Builder)
	if e.err != nil {
		if e.err != errPanic {
			fmt.Fprintf(w, "\t- returns error: %v\n", e.err)
		} else {
			fmt.Fprintf(w, "\t- panics with: %v\n", e.panicArgument)
		}
	}
	if e.plannedDelay > 0 {
		fmt.Fprintf(w, "\t- delayed execution for: %v\n", e.plannedDelay)
	}
	if e.optional {
		fmt.Fprint(w, "\t- execution is optional\n")
	}
	if e.plannedCalls > 0 {
		fmt.Fprintf(w, "\t- execution calls awaited: %d\n", e.plannedCalls)
	}
	return w.String()
}

// queryBasedExpectation is a base class that adds a query matching logic
type queryBasedExpectation struct {
	expectSQL          string
	expectRewrittenSQL string
	args               []any
}

// queryOption identifies a kind of pgx option value that may be passed ahead of
// the real query arguments.
type queryOption uint8

const (
	optRewriter queryOption = 1 << iota
	optExecMode
	optResultFormats

	// optsQuery, optsExec and optsBatch mirror the option loops in pgx's
	// Conn.Query, Conn.exec and Conn.SendBatch respectively. The sets differ,
	// and an option a method does not consume stays a plain query argument.
	optsQuery = optRewriter | optExecMode | optResultFormats
	optsExec  = optRewriter | optExecMode
	optsBatch = optRewriter
)

// splitQueryOptions consumes the leading pgx option values that a query method
// accepts before its real arguments and returns the query rewriter among them,
// if any, together with the remaining arguments. It mirrors the option loop
// pgx runs, so that an expectation only has to describe the actual query
// parameters and never the options.
func splitQueryOptions(args []any, allowed queryOption) (rewriter pgx.QueryRewriter, rest []any) {
	rest = args
optionLoop:
	for len(rest) > 0 {
		switch arg := rest[0].(type) {
		case pgx.QueryResultFormats:
			if allowed&optResultFormats == 0 {
				break optionLoop
			}
		case pgx.QueryResultFormatsByOID:
			if allowed&optResultFormats == 0 {
				break optionLoop
			}
		case pgx.QueryExecMode:
			if allowed&optExecMode == 0 {
				break optionLoop
			}
		case pgx.QueryRewriter:
			if allowed&optRewriter == 0 {
				break optionLoop
			}
			rewriter = arg
		default:
			break optionLoop
		}
		rest = rest[1:]
	}
	return rewriter, rest
}

// rewrite applies rewriter, if there is one, the way pgx would.
func rewrite(rewriter pgx.QueryRewriter, sql string, args []any) (string, []any, error) {
	if rewriter == nil {
		return "", args, nil
	}
	// note: pgx.Conn is not currently used by the query rewriter
	return rewriter.RewriteQuery(context.Background(), nil, sql, args)
}

func (e *queryBasedExpectation) argsMatches(sql string, args []any, allowed queryOption) (rewrittenSQL string, err error) {
	rewriter, args := splitQueryOptions(args, allowed)
	if rewrittenSQL, args, err = rewrite(rewriter, sql, args); err != nil {
		return rewrittenSQL, fmt.Errorf("error rewriting query: %w", err)
	}

	// the expectation may be written with a rewriter too, e.g.
	// WithArgs(pgx.NamedArgs{...}), in which case it is expanded the same way
	eRewriter, eargs := splitQueryOptions(e.args, allowed)
	if _, eargs, err = rewrite(eRewriter, sql, eargs); err != nil {
		return rewrittenSQL, fmt.Errorf("error rewriting query expectation: %w", err)
	}

	if len(args) != len(eargs) {
		return rewrittenSQL, fmt.Errorf("expected %d, but got %d arguments", len(eargs), len(args))
	}
	for k, v := range args {
		// custom argument matcher
		if matcher, ok := eargs[k].(Argument); ok {
			if !matcher.Match(v) {
				return rewrittenSQL, fmt.Errorf("matcher %T could not match %d argument %T - %+v", matcher, k, args[k], args[k])
			}
			continue
		}
		if darg := eargs[k]; !reflect.DeepEqual(darg, v) {
			return rewrittenSQL, fmt.Errorf("argument %d expected [%T - %+v] does not match actual [%T - %+v]", k, darg, darg, v, v)
		}
	}
	return
}

// ExpectedClose is used to manage pgx.Close expectation
// returned by pgxmock.ExpectClose
type ExpectedClose struct {
	commonExpectation
}

// String returns string representation
func (e *ExpectedClose) String() string {
	return "ExpectedClose => expecting call to Close()\n" + e.commonExpectation.String()
}

// ExpectedBegin is used to manage *pgx.Begin expectation
// returned by pgxmock.ExpectBegin.
type ExpectedBegin struct {
	commonExpectation
	opts pgx.TxOptions
}

// String returns string representation
func (e *ExpectedBegin) String() string {
	msg := "ExpectedBegin => expecting call to Begin() or to BeginTx()\n"
	if e.opts != (pgx.TxOptions{}) {
		msg += fmt.Sprintf("\t- transaction options awaited: %+v\n", e.opts)
	}
	return msg + e.commonExpectation.String()
}

// ExpectedCommit is used to manage pgx.Tx.Commit expectation
// returned by pgxmock.ExpectCommit.
type ExpectedCommit struct {
	commonExpectation
}

// String returns string representation
func (e *ExpectedCommit) String() string {
	return "ExpectedCommit => expecting call to Tx.Commit()\n" + e.commonExpectation.String()
}

// ExpectedExec is used to manage pgx.Conn.Exec and pgx.Tx.Exec expectations.
// Returned by pgxmock.ExpectExec.
type ExpectedExec struct {
	commonExpectation
	queryBasedExpectation
	result pgconn.CommandTag
}

// WithArgs will match given expected args to actual database exec operation arguments.
// if at least one argument does not match, it will return an error. For specific
// arguments an pgxmock.Argument interface can be used to match an argument.
func (e *ExpectedExec) WithArgs(args ...any) *ExpectedExec {
	e.args = args
	return e
}

// WithRewrittenSQL will match given expected expression to a rewritten SQL statement by
// an pgx.QueryRewriter argument
func (e *ExpectedExec) WithRewrittenSQL(sql string) *ExpectedExec {
	e.expectRewrittenSQL = sql
	return e
}

// String returns string representation
func (e *ExpectedExec) String() string {
	var msg strings.Builder
	msg.WriteString("ExpectedExec => expecting call to Exec():\n")
	fmt.Fprintf(&msg, "\t- matches sql: '%s'\n", e.expectSQL)

	if len(e.args) == 0 {
		msg.WriteString("\t- is without arguments\n")
	} else {
		msg.WriteString("\t- is with arguments:\n")
		for i, arg := range e.args {
			fmt.Fprintf(&msg, "\t\t%d - %+v\n", i, arg)
		}
	}
	if e.result.String() != "" {
		fmt.Fprintf(&msg, "\t- returns result: %s\n", e.result)
	}

	return msg.String() + e.commonExpectation.String()
}

// WillReturnResult arranges for an expected Exec() to return a particular
// result, there is pgxmock.NewResult(op string, rowsAffected int64) method
// to build a corresponding result.
func (e *ExpectedExec) WillReturnResult(result pgconn.CommandTag) *ExpectedExec {
	e.result = result
	return e
}

// ExpectedBatch is used to manage pgx.Batch expectations.
// Returned by pgxmock.ExpectBatch.
type ExpectedBatch struct {
	commonExpectation
	mock            *pgxmock
	expectedQueries []*queryBasedExpectation
	closed          bool
	mustBeClosed    bool
}

// ExpectExec allows to expect Queue().Exec() on this batch.
func (e *ExpectedBatch) ExpectExec(query string) *ExpectedExec {
	ee := &ExpectedExec{}
	ee.expectSQL = query
	e.expectedQueries = append(e.expectedQueries, &ee.queryBasedExpectation)
	return addExpectation(e.mock, ee)
}

// ExpectQuery allows to expect Queue().Query() or Queue().QueryRow() on this batch.
func (e *ExpectedBatch) ExpectQuery(query string) *ExpectedQuery {
	eq := &ExpectedQuery{}
	eq.expectSQL = query
	e.expectedQueries = append(e.expectedQueries, &eq.queryBasedExpectation)
	return addExpectation(e.mock, eq)
}

// String returns string representation
func (e *ExpectedBatch) String() string {
	msg := "ExpectedBatch => expecting call to SendBatch()\n"
	if e.mustBeClosed {
		msg += "\t- batch must be closed\n"
	}
	return msg + e.commonExpectation.String()
}

// ExpectedPrepare is used to manage pgx.Prepare or pgx.Tx.Prepare expectations.
// Returned by pgxmock.ExpectPrepare.
type ExpectedPrepare struct {
	commonExpectation
	expectStmtName string
	expectSQL      string
}

// String returns string representation
func (e *ExpectedPrepare) String() string {
	msg := "ExpectedPrepare => expecting call to Prepare():\n"
	msg += fmt.Sprintf("\t- matches statement name: '%s'\n", e.expectStmtName)
	msg += fmt.Sprintf("\t- matches sql: '%s'\n", e.expectSQL)
	return msg + e.commonExpectation.String()
}

// ExpectedDeallocate is used to manage pgx.Deallocate and pgx.DeallocateAll expectations.
// Returned by pgxmock.ExpectDeallocate(string) and pgxmock.ExpectDeallocateAll().
type ExpectedDeallocate struct {
	commonExpectation
	expectStmtName string
	expectAll      bool
}

// String returns string representation
func (e *ExpectedDeallocate) String() string {
	msg := "ExpectedDeallocate => expecting call to Deallocate():\n"
	if e.expectAll {
		msg += "\t- matches all statements\n"
	} else {
		msg += fmt.Sprintf("\t- matches statement name: '%s'\n", e.expectStmtName)
	}
	return msg + e.commonExpectation.String()
}

// ExpectedPing is used to manage Ping() expectations
type ExpectedPing struct {
	commonExpectation
}

// String returns string representation
func (e *ExpectedPing) String() string {
	msg := "ExpectedPing => expecting call to Ping()\n"
	return msg + e.commonExpectation.String()
}

// ExpectedQuery is used to manage pgx.Conn.Query, pgx.Conn.QueryRow,
// pgx.Tx.Query and pgx.Tx.QueryRow expectations
type ExpectedQuery struct {
	commonExpectation
	queryBasedExpectation
	rows             pgx.Rows
	rowsMustBeClosed bool
	rowsWereClosed   atomic.Bool
}

// WithArgs will match given expected args to actual database query arguments.
// if at least one argument does not match, it will return an error. For specific
// arguments an pgxmock.Argument interface can be used to match an argument.
func (e *ExpectedQuery) WithArgs(args ...any) *ExpectedQuery {
	e.args = args
	return e
}

// WithRewrittenSQL will match given expected expression to a rewritten SQL statement by
// an pgx.QueryRewriter argument
func (e *ExpectedQuery) WithRewrittenSQL(sql string) *ExpectedQuery {
	e.expectRewrittenSQL = sql
	return e
}

// RowsWillBeClosed expects this query rows to be closed.
func (e *ExpectedQuery) RowsWillBeClosed() *ExpectedQuery {
	e.rowsMustBeClosed = true
	return e
}

// String returns string representation
func (e *ExpectedQuery) String() string {
	var msg strings.Builder
	msg.WriteString("ExpectedQuery => expecting call to Query() or to QueryRow():\n")
	fmt.Fprintf(&msg, "\t- matches sql: '%s'\n", e.expectSQL)

	if len(e.args) == 0 {
		msg.WriteString("\t- is without arguments\n")
	} else {
		msg.WriteString("\t- is with arguments:\n")
		for i, arg := range e.args {
			fmt.Fprintf(&msg, "\t\t%d - %+v\n", i, arg)
		}
	}
	if e.rows != nil {
		fmt.Fprintf(&msg, "%s\n", e.rows)
	}
	return msg.String() + e.commonExpectation.String()
}

// WillReturnRows specifies the set of resulting rows that will be returned
// by the triggered query
func (e *ExpectedQuery) WillReturnRows(rows ...*Rows) *ExpectedQuery {
	e.rows = &rowSets{sets: rows, ex: e}
	return e
}

// freshRows returns an independent view over the expected rows. Every call to
// the mocked Query() gets its own cursor, so an expectation reused via Times()
// (or matched repeatedly out of order) returns the full result set each time
// instead of an already exhausted one.
func (e *ExpectedQuery) freshRows() pgx.Rows {
	src, ok := e.rows.(*rowSets)
	if !ok {
		return e.rows
	}
	sets := make([]*Rows, len(src.sets))
	for i, s := range src.sets {
		sets[i] = s.clone()
	}
	return &rowSets{sets: sets, RowSetNo: src.RowSetNo, ex: e}
}

// ExpectedCopyFrom is used to manage *pgx.Conn.CopyFrom expectations.
// Returned by *pgxmock.ExpectCopyFrom.
type ExpectedCopyFrom struct {
	commonExpectation
	expectedTableName pgx.Identifier
	expectedColumns   []string
	rowsAffected      int64
}

// String returns string representation
func (e *ExpectedCopyFrom) String() string {
	msg := "ExpectedCopyFrom => expecting CopyFrom which:"
	msg += "\n  - matches table name: '" + e.expectedTableName.Sanitize() + "'"
	msg += fmt.Sprintf("\n  - matches column names: '%+v'", e.expectedColumns)

	if e.err != nil {
		msg += fmt.Sprintf("\n  - should returns error: %s", e.err)
	}

	return msg
}

// WillReturnResult arranges for an expected CopyFrom() to return a number of rows affected
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

// ExpectedRollback is used to manage pgx.Tx.Rollback expectation
// returned by pgxmock.ExpectRollback.
type ExpectedRollback struct {
	commonExpectation
}

// String returns string representation
func (e *ExpectedRollback) String() string {
	msg := "ExpectedRollback => expecting transaction Rollback"
	if e.err != nil {
		msg += fmt.Sprintf(", which should return error: %s", e.err)
	}
	return msg
}
