package pgxmock

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// BatchElement helps to create batches for testing
type BatchElement struct {
	sql          string
	rewrittenSQL string
	args         []interface{}
}

// NewBatchElement creates new query that will be queued in batch.
// Function accepts sql string and optionally arguments
func NewBatchElement(sql string, args ...interface{}) *BatchElement {
	return &BatchElement{
		sql:  sql,
		args: args,
	}
}

// WithRewrittenSQL will match given expected expression to a rewritten SQL statement by
// a pgx.QueryRewriter argument
func (be *BatchElement) WithRewrittenSQL(sql string) *BatchElement {
	be.rewrittenSQL = sql
	return be
}

// Batch is a batch mock that helps to create batches for testing
type Batch struct {
	elements []*BatchElement
}

// AddBatchElements adds any number of BatchElement to Batch
// that is used for mocking pgx.SendBatch()
func (b *Batch) AddBatchElements(bes ...*BatchElement) *Batch {
	for _, be := range bes {
		b.elements = append(b.elements, be)
	}
	return b
}

// NewBatch creates a structure that helps to combine multiple queries
// that will be used in batches. This function should be used in .ExpectSendBatch()
func NewBatch() *Batch {
	return &Batch{}
}

// batchResults is a subsidiary structure for mocking BatchResults interface response
type batchResults struct {
	br *BatchResults
	ex *ExpectedBatch
}

// Query is a mock for Query() function in pgx.BatchResults interface
func (b *batchResults) Query() (pgx.Rows, error) {
	if b.br.queryErr != nil {
		return nil, b.br.queryErr
	}
	rs := &rowSets{sets: []*Rows{b.br.rows}}
	rs.Next()
	return rs, nil
}

// Exec is a mock for Exec() function in pgx.BatchResults interface
func (b *batchResults) Exec() (pgconn.CommandTag, error) {
	if b.br.execErr != nil {
		return pgconn.CommandTag{}, b.br.execErr
	}
	return b.br.commandTag, nil
}

// QueryRow is a mock for QueryRow() function in pgx.BatchResults interface
func (b *batchResults) QueryRow() pgx.Row {
	rs := &rowSets{sets: []*Rows{b.br.rows}}
	rs.Next()
	return rs
}

// Close is a mock for Close() function in pgx.BatchResults interface
func (b *batchResults) Close() error {
	b.ex.batchWasClosed = true
	return b.br.closeErr
}

// BatchResults is a subsidiary structure for mocking SendBatch() function
// response. There is an option to mock returned Rows, errors and commandTag
type BatchResults struct {
	commandTag pgconn.CommandTag
	rows       *Rows
	queryErr   error
	execErr    error
	closeErr   error
}

// NewBatchResults returns a mock response for SendBatch() function
func NewBatchResults() *BatchResults {
	return &BatchResults{}
}

// QueryError sets the error that will be returned by Query() function
// called using pgx.BatchResults interface
func (b *BatchResults) QueryError(err error) *BatchResults {
	b.queryErr = err
	return b
}

// ExecError sets the error that will be returned by Exec() function
// called using pgx.BatchResults interface
func (b *BatchResults) ExecError(err error) *BatchResults {
	b.execErr = err
	return b
}

// CloseError sets the error that will be returned by Close() function
// called using pgx.BatchResults interface
func (b *BatchResults) CloseError(err error) *BatchResults {
	b.closeErr = err
	return b
}

// WillReturnRows allows to return mocked Rows by Query() and QueryRow()
// functions in pgx.BatchResults interface
func (b *BatchResults) WillReturnRows(rows *Rows) *BatchResults {
	b.rows = rows
	return b
}

// AddCommandTag allows to add pgconn.CommandTag to batchResults struct
// that will be returned in Exec() function
func (b *BatchResults) AddCommandTag(ct pgconn.CommandTag) *BatchResults {
	b.commandTag = ct
	return b
}
