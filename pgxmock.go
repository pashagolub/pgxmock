/*
package pgxmock is a mock library implementing pgx connector. Which has one and only
purpose - to simulate pgx driver behavior in tests, without needing a real
database connection. It helps to maintain correct **TDD** workflow.

It does not require (almost) any modifications to your source code in order to test
and mock database operations. Supports concurrency and multiple database mocking.

The driver allows to mock any pgx driver method behavior.
*/
package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

// pgxMockIface interface serves to create expectations
// for any kind of database action in order to mock
// and test real database behavior.
type pgxMockIface interface {
	// ExpectClose queues an expectation for this database
	// action to be triggered. the *ExpectedClose allows
	// to mock database response
	ExpectClose() *ExpectedClose

	// ExpectationsWereMet checks whether all queued expectations
	// were met in order (unless MatchExpectationsInOrder set to false).
	// If any of them was not met - an error is returned.
	ExpectationsWereMet() error

	// ExpectPrepare expects Prepare() to be called with expectedSQL query.
	// the *ExpectedPrepare allows to mock database response.
	// Note that you may expect Query() or Exec() on the *ExpectedPrepare
	// statement to prevent repeating expectedSQL
	ExpectPrepare(expectedStmtName, expectedSQL string) *ExpectedPrepare

	// ExpectQuery expects Query() or QueryRow() to be called with expectedSQL query.
	// the *ExpectedQuery allows to mock database response.
	ExpectQuery(expectedSQL string) *ExpectedQuery

	// ExpectExec expects Exec() to be called with expectedSQL query.
	// the *ExpectedExec allows to mock database response
	ExpectExec(expectedSQL string) *ExpectedExec

	// ExpectBegin expects pgx.Conn.Begin to be called.
	// the *ExpectedBegin allows to mock database response
	ExpectBegin() *ExpectedBegin

	// ExpectBeginTx expects expects BeginTx() to be called with expectedSQL
	// query. The *ExpectedBegin allows to mock database response.
	ExpectBeginTx(txOptions pgx.TxOptions) *ExpectedBegin

	// ExpectCommit expects pgx.Tx.Commit to be called.
	// the *ExpectedCommit allows to mock database response
	ExpectCommit() *ExpectedCommit

	// ExpectReset expects pgxpool.Reset() to be called.
	// The *ExpectedReset allows to mock database response
	ExpectReset() *ExpectedReset

	// ExpectRollback expects pgx.Tx.Rollback to be called.
	// the *ExpectedRollback allows to mock database response
	ExpectRollback() *ExpectedRollback

	// ExpectPing expected pgx.Conn.Ping to be called.
	// the *ExpectedPing allows to mock database response
	//
	// Ping support only exists in the SQL library in Go 1.8 and above.
	// ExpectPing in Go <=1.7 will return an ExpectedPing but not register
	// any expectations.
	//
	// You must enable pings using MonitorPingsOption for this to register
	// any expectations.
	ExpectPing() *ExpectedPing

	// ExpectCopyFrom expects pgx.CopyFrom to be called.
	// the *ExpectCopyFrom allows to mock database response
	ExpectCopyFrom(expectedTableName pgx.Identifier, expectedColumns []string) *ExpectedCopyFrom

	// MatchExpectationsInOrder gives an option whether to match all
	// expectations in the order they were set or not.
	//
	// By default it is set to - true. But if you use goroutines
	// to parallelize your query executation, that option may
	// be handy.
	//
	// This option may be turned on anytime during tests. As soon
	// as it is switched to false, expectations will be matched
	// in any order. Or otherwise if switched to true, any unmatched
	// expectations will be expected in order
	MatchExpectationsInOrder(bool)

	// NewRows allows Rows to be created from a
	// sql driver.Value slice or from the CSV string and
	// to be used as sql driver.Rows.
	NewRows(columns []string) *Rows

	// NewRowsWithColumnDefinition allows Rows to be created from a
	// sql driver.Value slice with a definition of sql metadata
	NewRowsWithColumnDefinition(columns ...pgconn.FieldDescription) *Rows

	// New Column allows to create a Column
	NewColumn(name string) *pgconn.FieldDescription

	Config() *pgxpool.Config

	PgConn() *pgconn.PgConn
}

type pgxIface interface {
	pgxMockIface
	Begin(context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Ping(context.Context) error
	Prepare(context.Context, string, string) (*pgconn.StatementDescription, error)
	PgConn() *pgconn.PgConn
}

type PgxConnIface interface {
	pgxIface
	pgx.Tx
	Close(ctx context.Context) error
	Deallocate(ctx context.Context, name string) error
}

type PgxPoolIface interface {
	pgxIface
	pgx.Tx
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
	AcquireAllIdle(ctx context.Context) []*pgxpool.Conn
	AcquireFunc(ctx context.Context, f func(*pgxpool.Conn) error) error
	AsConn() PgxConnIface
	Close()
	Stat() *pgxpool.Stat
	Reset()
}

type pgxmock struct {
	ordered      bool
	queryMatcher QueryMatcher
	monitorPings bool

	expected []expectation
}

func (c *pgxmock) Config() *pgxpool.Config {
	return &pgxpool.Config{}
}

func (c *pgxmock) AcquireAllIdle(_ context.Context) []*pgxpool.Conn {
	return []*pgxpool.Conn{}
}

func (c *pgxmock) AcquireFunc(_ context.Context, _ func(*pgxpool.Conn) error) error {
	return nil
}

// region Expectations
func (c *pgxmock) ExpectClose() *ExpectedClose {
	e := &ExpectedClose{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) MatchExpectationsInOrder(b bool) {
	c.ordered = b
}

func (c *pgxmock) ExpectationsWereMet() error {
	for _, e := range c.expected {
		e.Lock()
		fulfilled := e.fulfilled()
		e.Unlock()

		if !fulfilled {
			return fmt.Errorf("there is a remaining expectation which was not matched: %s", e)
		}

		// for expected prepared statement check whether it was closed if expected
		if prep, ok := e.(*ExpectedPrepare); ok {
			if prep.mustBeClosed && !prep.wasClosed {
				return fmt.Errorf("expected prepared statement to be closed, but it was not: %s", prep)
			}
		}

		// must check whether all expected queried rows are closed
		if query, ok := e.(*ExpectedQuery); ok {
			if query.rowsMustBeClosed && !query.rowsWereClosed {
				return fmt.Errorf("expected query rows to be closed, but it was not: %s", query)
			}
		}
	}
	return nil
}

func (c *pgxmock) ExpectQuery(expectedSQL string) *ExpectedQuery {
	e := &ExpectedQuery{}
	e.expectSQL = expectedSQL
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectCommit() *ExpectedCommit {
	e := &ExpectedCommit{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectRollback() *ExpectedRollback {
	e := &ExpectedRollback{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectBegin() *ExpectedBegin {
	e := &ExpectedBegin{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectBeginTx(txOptions pgx.TxOptions) *ExpectedBegin {
	e := &ExpectedBegin{opts: txOptions}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectExec(expectedSQL string) *ExpectedExec {
	e := &ExpectedExec{}
	e.expectSQL = expectedSQL
	// e.converter = c.converter
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectCopyFrom(expectedTableName pgx.Identifier, expectedColumns []string) *ExpectedCopyFrom {
	e := &ExpectedCopyFrom{}
	e.expectedTableName = expectedTableName
	e.expectedColumns = expectedColumns
	c.expected = append(c.expected, e)
	return e
}

// ExpectReset expects Reset to be called.
func (c *pgxmock) ExpectReset() *ExpectedReset {
	e := &ExpectedReset{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectPing() *ExpectedPing {
	if !c.monitorPings {
		log.Println("ExpectPing will have no effect as monitoring pings is disabled. Use MonitorPingsOption to enable.")
		return nil
	}
	e := &ExpectedPing{}
	c.expected = append(c.expected, e)
	return e
}

func (c *pgxmock) ExpectPrepare(expectedStmtName, expectedSQL string) *ExpectedPrepare {
	e := &ExpectedPrepare{expectSQL: expectedSQL, expectStmtName: expectedStmtName, mock: c}
	c.expected = append(c.expected, e)
	return e
}

//endregion Expectations

// NewRows allows Rows to be created from a
// atring slice or from the CSV string and
// to be used as sql driver.Rows.
func (c *pgxmock) NewRows(columns []string) *Rows {
	r := NewRows(columns)
	return r
}

// PgConn exposes the underlying low level postgres connection
// This is just here to support interfaces that use it. Here is just returns an empty PgConn
func (c *pgxmock) PgConn() *pgconn.PgConn {
	p := pgconn.PgConn{}
	return &p
}

// NewRowsWithColumnDefinition allows Rows to be created from a
// sql driver.Value slice with a definition of sql metadata
func (c *pgxmock) NewRowsWithColumnDefinition(columns ...pgconn.FieldDescription) *Rows {
	r := NewRowsWithColumnDefinition(columns...)
	return r
}

// NewColumn allows to create a Column that can be enhanced with metadata
// using OfType/Nullable/WithLength/WithPrecisionAndScale methods.
func (c *pgxmock) NewColumn(name string) *pgconn.FieldDescription {
	return &pgconn.FieldDescription{Name: name}
}

// open a mock database driver connection
func (c *pgxmock) open(options []func(*pgxmock) error) error {
	for _, option := range options {
		err := option(c)
		if err != nil {
			return err
		}
	}
	// if c.converter == nil {
	// 	c.converter = driver.DefaultParameterConverter
	// }
	if c.queryMatcher == nil {
		c.queryMatcher = QueryMatcherRegexp
	}

	if c.monitorPings {
		// We call Ping on the driver shortly to verify startup assertions by
		// driving internal behaviour of the sql standard library. We don't
		// want this call to ping to be monitored for expectation purposes so
		// temporarily disable.
		c.monitorPings = false
		defer func() { c.monitorPings = true }()
	}
	return c.Ping(context.TODO())
}

// Close a mock database driver connection. It may or may not
// be called depending on the circumstances, but if it is called
// there must be an *ExpectedClose expectation satisfied.
func (c *pgxmock) close(context.Context) error {
	var expected *ExpectedClose
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedClose); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return fmt.Errorf("call to database Close, was not expected, next expectation is: %s", next)
		}
	}

	if expected == nil {
		msg := "call to database Close was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	return expected.err
}

func (c *pgxmock) Conn() *pgx.Conn {
	panic("Conn() is not available in pgxmock")
}

func (c *pgxmock) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, _ pgx.CopyFromSource) (int64, error) {
	ex, err := c.copyFrom(tableName, columnNames)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if err != nil {
				return ex.rowsAffected, err
			}
			return ex.rowsAffected, nil
		case <-ctx.Done():
			return -1, ErrCancelled
		}
	}
	return -1, err
}

func (c *pgxmock) copyFrom(tableName pgx.Identifier, columnNames []string) (*ExpectedCopyFrom, error) {
	var expected *ExpectedCopyFrom
	var fulfilled int
	var ok bool

	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedCopyFrom); ok {
				break
			}

			next.Unlock()
			return nil, fmt.Errorf("call to CopyFrom statement with table name '%s', was not expected, next expectation is: %s", tableName, next)
		}

		if pr, ok := next.(*ExpectedCopyFrom); ok {
			if reflect.DeepEqual(pr.expectedTableName, tableName) && reflect.DeepEqual(pr.expectedColumns, columnNames) {
				expected = pr
				break
			}
		}
		next.Unlock()
	}

	if expected == nil {
		msg := "call to CopyFrom table name '%s' was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, tableName)
	}
	defer expected.Unlock()
	if !reflect.DeepEqual(expected.expectedTableName, tableName) {
		return nil, fmt.Errorf("CopyFrom: table name '%s' was not expected, expected table name is '%s'", tableName, expected.expectedTableName)
	}
	if !reflect.DeepEqual(expected.expectedColumns, columnNames) {
		return nil, fmt.Errorf("CopyFrom: column names '%v' were not expected, expected column names are '%v'", columnNames, expected.expectedColumns)
	}

	expected.triggered = true
	return expected, expected.err
}

func (c *pgxmock) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (c *pgxmock) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (c *pgxmock) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	ex, err := c.begin(txOptions)
	if ex != nil {
		time.Sleep(ex.delay)
	}
	if err != nil {
		return nil, err
	}

	return c, ctx.Err()
}

func (c *pgxmock) Begin(ctx context.Context) (pgx.Tx, error) {
	return c.BeginTx(ctx, pgx.TxOptions{})
}

func (c *pgxmock) begin(txOptions pgx.TxOptions) (*ExpectedBegin, error) {
	var expected *ExpectedBegin
	var ok bool
	var fulfilled int
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedBegin); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return nil, fmt.Errorf("call to database transaction Begin, was not expected, next expectation is: %s", next)
		}
	}
	if expected == nil {
		msg := "call to database transaction Begin was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg)
	}
	defer expected.Unlock()
	if expected.opts != txOptions {
		return nil, fmt.Errorf("Begin: call with transaction options '%v' was not expected, expected name is '%v'", txOptions, expected.opts)
	}
	expected.triggered = true

	return expected, expected.err
}

func (c *pgxmock) Prepare(ctx context.Context, name, query string) (*pgconn.StatementDescription, error) {
	ex, err := c.prepare(name, query)
	if ex != nil {
		time.Sleep(ex.delay)
	}
	if err != nil {
		return nil, err
	}

	return &pgconn.StatementDescription{Name: name, SQL: query}, ctx.Err()
}

func (c *pgxmock) prepare(name string, query string) (*ExpectedPrepare, error) {
	var expected *ExpectedPrepare
	var fulfilled int
	var ok bool

	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedPrepare); ok {
				break
			}

			next.Unlock()
			return nil, fmt.Errorf("call to Prepare statement with query '%s', was not expected, next expectation is: %s", query, next)
		}

		if pr, ok := next.(*ExpectedPrepare); ok {
			if err := c.queryMatcher.Match(pr.expectSQL, query); err == nil {
				expected = pr
				break
			}
		}
		next.Unlock()
	}

	if expected == nil {
		msg := "call to Prepare '%s' query was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, query)
	}
	defer expected.Unlock()
	if expected.expectStmtName != name {
		return nil, fmt.Errorf("Prepare: prepared statement name '%s' was not expected, expected name is '%s'", name, expected.expectStmtName)
	}
	if err := c.queryMatcher.Match(expected.expectSQL, query); err != nil {
		return nil, fmt.Errorf("Prepare: %v", err)
	}

	expected.triggered = true
	return expected, expected.err
}

func (c *pgxmock) Deallocate(ctx context.Context, name string) error {
	var expected *ExpectedPrepare
	for _, next := range c.expected {
		next.Lock()
		if pr, ok := next.(*ExpectedPrepare); ok && pr.expectStmtName == name {
			expected = pr
			next.Unlock()
			break
		}
		next.Unlock()
	}
	if expected == nil {
		return fmt.Errorf("Deallocate: prepared statement name '%s' doesn't exist", name)
	}
	expected.wasClosed = true
	return ctx.Err()
}

func (c *pgxmock) Commit(ctx context.Context) error {
	var expected *ExpectedCommit
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedCommit); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return fmt.Errorf("call to Commit transaction, was not expected, next expectation is: %s", next)
		}
	}
	if expected == nil {
		msg := "call to Commit transaction was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	if expected.err != nil {
		return expected.err
	}
	return ctx.Err()
}

func (c *pgxmock) Rollback(ctx context.Context) error {
	var expected *ExpectedRollback
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedRollback); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return fmt.Errorf("call to Rollback transaction, was not expected, next expectation is: %s", next)
		}
	}
	if expected == nil {
		msg := "call to Rollback transaction was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	if expected.err != nil {
		return expected.err
	}
	return ctx.Err()
}

// ErrCancelled defines an error value, which can be expected in case of
// such cancellation error.
var ErrCancelled = errors.New("canceling query due to user request")

// Implement the "QueryerContext" interface
func (c *pgxmock) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	ex, err := c.query(sql, args)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if err != nil {
				return nil, err
			}
			return ex.rows, nil
		case <-ctx.Done():
			return nil, ErrCancelled
		}
	}

	return nil, err
}

func (c *pgxmock) query(query string, args []interface{}) (*ExpectedQuery, error) {
	var expected *ExpectedQuery
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedQuery); ok {
				break
			}
			next.Unlock()
			return nil, fmt.Errorf("call to Query '%s' with args %+v, was not expected, next expectation is: %s", query, args, next)
		}
		if qr, ok := next.(*ExpectedQuery); ok {
			if err := c.queryMatcher.Match(qr.expectSQL, query); err != nil {
				next.Unlock()
				continue
			}
			if err := qr.attemptArgMatch(args); err == nil {
				expected = qr
				break
			}
		}
		next.Unlock()
	}

	if expected == nil {
		msg := "call to Query '%s' with args %+v was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, query, args)
	}

	defer expected.Unlock()

	if err := c.queryMatcher.Match(expected.expectSQL, query); err != nil {
		return nil, fmt.Errorf("Query: %v", err)
	}

	if err := expected.argsMatches(args); err != nil {
		return nil, fmt.Errorf("Query '%s', arguments do not match: %s", query, err)
	}

	expected.triggered = true
	if expected.err != nil {
		return expected, expected.err // mocked to return error
	}

	if expected.rows == nil {
		return nil, fmt.Errorf("Query '%s' with args %+v, must return a pgx.Rows, but it was not set for expectation %T as %+v", query, args, expected, expected)
	}
	return expected, nil
}

type errRow struct {
	err error
}

func (er errRow) Scan(...interface{}) error {
	return er.err
}

func (c *pgxmock) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	ex, err := c.query(sql, args)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if (err != nil) || (ex.rows == nil) {
				return errRow{err}
			}
			_ = ex.rows.Next()
			return ex.rows
		case <-ctx.Done():
			return errRow{ctx.Err()}
		}
	}
	return errRow{err}
}

// Implement the "ExecerContext" interface
func (c *pgxmock) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	ex, err := c.exec(query, args)
	if ex != nil {
		select {
		case <-time.After(ex.delay):
			if err != nil {
				return pgconn.NewCommandTag(""), err
			}
			return ex.result, nil
		case <-ctx.Done():
			return pgconn.NewCommandTag(""), ErrCancelled
		}
	}
	return pgconn.NewCommandTag(""), err
}

func (c *pgxmock) exec(query string, args []interface{}) (*ExpectedExec, error) {
	var expected *ExpectedExec
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if c.ordered {
			if expected, ok = next.(*ExpectedExec); ok {
				break
			}
			next.Unlock()
			return nil, fmt.Errorf("call to ExecQuery '%s' with args %+v, was not expected, next expectation is: %s", query, args, next)
		}
		if exec, ok := next.(*ExpectedExec); ok {
			if err := c.queryMatcher.Match(exec.expectSQL, query); err != nil {
				next.Unlock()
				continue
			}

			if err := exec.attemptArgMatch(args); err == nil {
				expected = exec
				break
			}
		}
		next.Unlock()
	}
	if expected == nil {
		msg := "call to ExecQuery '%s' with args %+v was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg, query, args)
	}
	defer expected.Unlock()

	if err := c.queryMatcher.Match(expected.expectSQL, query); err != nil {
		return nil, fmt.Errorf("ExecQuery: %v", err)
	}

	if err := expected.argsMatches(args); err != nil {
		return nil, fmt.Errorf("ExecQuery '%s', arguments do not match: %s", query, err)
	}

	expected.triggered = true
	if expected.err != nil {
		return expected, expected.err // mocked to return error
	}

	if expected.result.String() == "" {
		return nil, fmt.Errorf("ExecQuery '%s' with args %+v, must return a pgconn.CommandTag, but it was not set for expectation %T as %+v", query, args, expected, expected)
	}

	return expected, nil
}

// Implement the "Pinger" interface - the explicit DB driver ping was only added to database/sql in Go 1.8
func (c *pgxmock) Ping(ctx context.Context) error {
	if !c.monitorPings {
		return nil
	}

	ex, err := c.ping()
	if ex != nil {
		select {
		case <-ctx.Done():
			return ErrCancelled
		case <-time.After(ex.delay):
		}
	}

	return err
}

func (c *pgxmock) ping() (*ExpectedPing, error) {
	var expected *ExpectedPing
	var fulfilled int
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			fulfilled++
			continue
		}

		if expected, ok = next.(*ExpectedPing); ok {
			break
		}

		next.Unlock()
		if c.ordered {
			return nil, fmt.Errorf("call to database Ping, was not expected, next expectation is: %s", next)
		}
	}

	if expected == nil {
		msg := "call to database Ping was not expected"
		if fulfilled == len(c.expected) {
			msg = "all expectations were already fulfilled, " + msg
		}
		return nil, fmt.Errorf(msg)
	}

	expected.triggered = true
	expected.Unlock()
	return expected, expected.err
}

func (c *pgxmock) Reset() {
	var expected *ExpectedReset
	var ok bool
	for _, next := range c.expected {
		next.Lock()
		if next.fulfilled() {
			next.Unlock()
			continue
		}

		if expected, ok = next.(*ExpectedReset); ok {
			break
		}
		next.Unlock()
	}
	if expected == nil {
		return
	}
	expected.triggered = true
	expected.Unlock()
}
