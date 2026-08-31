package pgxmock

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Concurrent callers sharing one reused expectation must not race on the
// mocked rows or on the expectation bookkeeping.
func TestConcurrentQueriesOnReusedExpectation(t *testing.T) {
	const goroutines = 20

	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.MatchExpectationsInOrder(false)

	eq := mock.ExpectQuery("SELECT").
		WillReturnRows(NewRows([]string{"id"}).AddRow(1).AddRow(2)).
		RowsWillBeClosed()
	eq.Times(goroutines)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := mock.Query(context.Background(), "SELECT id FROM t")
			assert.NoError(t, err)
			n := 0
			for rows.Next() {
				var id any
				assert.NoError(t, rows.Scan(&id))
				n++
			}
			rows.Close()
			assert.Equal(t, 2, n)
		}()
	}
	wg.Wait()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ExpectationsWereMet may be read while queries are still in flight.
func TestConcurrentExpectationsWereMet(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.MatchExpectationsInOrder(false)

	eq := mock.ExpectQuery("SELECT").
		WillReturnRows(NewRows([]string{"id"}).AddRow(1)).
		RowsWillBeClosed()
	eq.Times(10)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := mock.Query(context.Background(), "SELECT id FROM t")
			assert.NoError(t, err)
			rows.Close()
		}()
	}
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = mock.ExpectationsWereMet()
		}()
	}
	wg.Wait()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Expectations declared up front may be matched from many goroutines at once,
// including while the expectation list is still growing from the test body.
func TestConcurrentMatchingWhileDeclaring(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)
	mock.MatchExpectationsInOrder(false)

	// Each expectation is fully configured before it is handed to a goroutine.
	var wg sync.WaitGroup
	for range 10 {
		mock.ExpectQuery("SELECT").
			WillReturnRows(NewRows([]string{"id"}).AddRow(1)).
			Maybe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rows, err := mock.Query(context.Background(), "SELECT id FROM t"); err == nil {
				rows.Close()
			}
		}()
	}
	wg.Wait()
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A connection obtained via AsConn shares the pool expectations.
func TestAsConnSharesExpectations(t *testing.T) {
	pool, err := NewPool(QueryMatcherOption(QueryMatcherAny))
	assert.NoError(t, err)

	conn := pool.AsConn()
	conn.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"id"}).AddRow(1))

	rows, err := pool.Query(context.Background(), "SELECT id FROM t")
	assert.NoError(t, err)
	rows.Close()
	assert.NoError(t, pool.ExpectationsWereMet())
	assert.NoError(t, conn.ExpectationsWereMet())
}
