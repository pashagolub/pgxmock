package pgxmock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// Modifiers used to return the CallModifier interface, so a chain only
// compiled when every type-specific builder came first. Both orders must work.
func TestModifiersChainInAnyOrder(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.MatchExpectationsInOrder(false)

	rows := func() *Rows { return NewRows([]string{"id"}).AddRow(1) }

	// modifier first, builder second
	mock.ExpectQuery("SELECT").
		Times(2).
		WithArgs(1).
		WillReturnRows(rows())

	// builder first, modifier second
	mock.ExpectQuery("SELECT").
		WithArgs(2).
		WillReturnRows(rows()).
		Times(2)

	for _, id := range []int{1, 1, 2, 2} {
		r, err := mock.Query(context.Background(), "SELECT id FROM t", id)
		assert.NoError(t, err)
		r.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Every expectation type keeps its concrete type through a modifier chain.
func TestModifiersPreserveConcreteType(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	// each assignment only compiles if the modifier returned the concrete type
	var (
		_ *ExpectedQuery      = mock.ExpectQuery("q").Maybe().Times(1)
		_ *ExpectedExec       = mock.ExpectExec("e").Maybe().WillDelayFor(0)
		_ *ExpectedBegin      = mock.ExpectBegin().Maybe()
		_ *ExpectedCommit     = mock.ExpectCommit().Maybe()
		_ *ExpectedRollback   = mock.ExpectRollback().Maybe()
		_ *ExpectedClose      = mock.ExpectClose().Maybe()
		_ *ExpectedPing       = mock.ExpectPing().Maybe()
		_ *ExpectedReset      = mock.ExpectReset().Maybe()
		_ *ExpectedPrepare    = mock.ExpectPrepare("s", "q").Maybe()
		_ *ExpectedDeallocate = mock.ExpectDeallocate("s").Maybe()
		_ *ExpectedBatch      = mock.ExpectBatch().Maybe()
		_ *ExpectedCopyFrom   = mock.ExpectCopyFrom(pgx.Identifier{"t"}, []string{"a"}).Maybe()
	)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// WillReturnError returned nothing at all, ending any chain it appeared in.
func TestWillReturnErrorIsChainable(t *testing.T) {
	errBoom := errors.New("boom")
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	mock.ExpectQuery("SELECT").
		WillReturnError(errBoom).
		WithArgs(1).
		Times(2)

	for range 2 {
		_, err := mock.Query(context.Background(), "SELECT id FROM t", 1)
		assert.ErrorIs(t, err, errBoom)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWillPanicIsChainable(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	mock.ExpectPing().WillPanic("boom").Maybe()

	assert.PanicsWithValue(t, "boom", func() {
		_ = mock.Ping(context.Background())
	})
}

// The modifiers still behave the way they did, only the return type changed.
func TestModifierSemanticsUnchanged(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	mock.ExpectPing().WillDelayFor(time.Second).Maybe().Times(4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.Error(t, mock.Ping(ctx), "a cancelled context must be honoured")
	assert.NoError(t, mock.ExpectationsWereMet(), "the call is optional")
}

// commonExpectation still satisfies the exported CallModifier interface.
var _ CallModifier = (*commonExpectation)(nil)
