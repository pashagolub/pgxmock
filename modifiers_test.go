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

// assertType is a compile-time check that v's static type is exactly T.
func assertType[T any](_ T) {}

// Every expectation type keeps its concrete type through a modifier chain.
func TestModifiersPreserveConcreteType(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	// assertType only compiles if the modifier chain's static type is exactly
	// the type argument - an interface return type (the pre-fix behavior)
	// would not be assignable and would fail to build.
	assertType[*ExpectedQuery](mock.ExpectQuery("q").Maybe().Times(1))
	assertType[*ExpectedExec](mock.ExpectExec("e").Maybe().WillDelayFor(0))
	assertType[*ExpectedBegin](mock.ExpectBegin().Maybe())
	assertType[*ExpectedCommit](mock.ExpectCommit().Maybe())
	assertType[*ExpectedRollback](mock.ExpectRollback().Maybe())
	assertType[*ExpectedClose](mock.ExpectClose().Maybe())
	assertType[*ExpectedPing](mock.ExpectPing().Maybe())
	assertType[*ExpectedReset](mock.ExpectReset().Maybe())
	assertType[*ExpectedPrepare](mock.ExpectPrepare("s", "q").Maybe())
	assertType[*ExpectedDeallocate](mock.ExpectDeallocate("s").Maybe())
	assertType[*ExpectedBatch](mock.ExpectBatch().Maybe())
	assertType[*ExpectedCopyFrom](mock.ExpectCopyFrom(pgx.Identifier{"t"}, []string{"a"}).Maybe())
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

// modifiable is the set of CallModifier methods every expectation type
// provides, each returning the expectation's own type.
type modifiable[T any] interface {
	Maybe() T
	Times(uint) T
	WillDelayFor(time.Duration) T
	WillReturnError(error) T
	WillPanic(any) T
}

// assertModifiersReturnReceiver checks that every modifier of e returns e.
func assertModifiersReturnReceiver[T modifiable[T]](t *testing.T, e T) {
	t.Helper()
	for _, got := range []T{e.Maybe(), e.Times(1), e.WillDelayFor(0), e.WillReturnError(nil), e.WillPanic(nil)} {
		assert.Same(t, any(e), any(got), "%T", e)
	}
}

func TestEveryModifierReturnsItsReceiver(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	assertModifiersReturnReceiver(t, mock.ExpectBatch())
	assertModifiersReturnReceiver(t, mock.ExpectBegin())
	assertModifiersReturnReceiver(t, mock.ExpectClose())
	assertModifiersReturnReceiver(t, mock.ExpectCommit())
	assertModifiersReturnReceiver(t, mock.ExpectCopyFrom(pgx.Identifier{"t"}, nil))
	assertModifiersReturnReceiver(t, mock.ExpectDeallocateAll())
	assertModifiersReturnReceiver(t, mock.ExpectExec("q"))
	assertModifiersReturnReceiver(t, mock.ExpectPing())
	assertModifiersReturnReceiver(t, mock.ExpectPrepare("s", "q"))
	assertModifiersReturnReceiver(t, mock.ExpectQuery("q"))
	assertModifiersReturnReceiver(t, mock.ExpectReset())
	assertModifiersReturnReceiver(t, mock.ExpectRollback())
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
