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

// NewBatchElement creates new query that will be queued in batch
// function accepts sql string and optionally arguments
func NewBatchElement(sql string, args ...interface{}) *BatchElement {
	return &BatchElement{
		sql:  sql,
		args: args,
	}
}

func (be *BatchElement) WithRewrittenSQL(sql string) *BatchElement {
	be.rewrittenSQL = sql
	return be
}

// Batch is a batch mock that helps to create batches for testing
type Batch struct {
	elements []*BatchElement
}

func (b *Batch) AddBatchElements(bes ...*BatchElement) *Batch {
	for _, be := range bes {
		b.elements = append(b.elements, be)
	}
	return b
}

// NewBatch creates a structure that helps to combine multiple queries that will be used in batches
// this function should be used in .ExpectSendBatch()
func NewBatch() *Batch {
	return &Batch{}
}

type batchResults struct {
	br *BatchResults
	ex *ExpectedBatch
}

func (b *batchResults) Query() (pgx.Rows, error) {
	if b.br.queryErr != nil {
		return nil, b.br.queryErr
	}
	rs := &rowSets{sets: []*Rows{b.br.rows}}
	rs.Next()
	return rs, nil
}

func (b *batchResults) Exec() (pgconn.CommandTag, error) {
	if b.br.execErr != nil {
		return pgconn.CommandTag{}, b.br.execErr
	}
	return b.br.commandTag, nil
}

func (b *batchResults) QueryRow() pgx.Row {
	rs := &rowSets{sets: []*Rows{b.br.rows}}
	rs.Next()
	return rs
}

func (b *batchResults) Close() error {
	b.ex.batchWasClosed = true
	return b.br.closeErr
}

type BatchResults struct {
	commandTag pgconn.CommandTag
	rows       *Rows
	queryErr   error
	execErr    error
	closeErr   error
}

func NewBatchResults() *BatchResults {
	return &BatchResults{}
}

func (b *BatchResults) QueryError(err error) *BatchResults {
	b.queryErr = err
	return b
}

func (b *BatchResults) ExecError(err error) *BatchResults {
	b.execErr = err
	return b
}

func (b *BatchResults) CloseError(err error) *BatchResults {
	b.closeErr = err
	return b
}

func (b *BatchResults) WillReturnRows(rows *Rows) *BatchResults {
	b.rows = rows
	return b
}

func (b *BatchResults) AddCommandTag(ct pgconn.CommandTag) *BatchResults {
	b.commandTag = ct
	return b
}
