package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
)

func cancelOrder(db pgxIface, orderID int) error {
	tx, _ := db.Begin(context.Background())
	_, _ = tx.Query(context.Background(), "SELECT * FROM orders {0} FOR UPDATE", orderID)
	err := tx.Rollback(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func TestIssue14EscapeSQL(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())
	mock.ExpectExec("INSERT INTO mytable\\(a, b\\)").
		WithArgs("A", "B").
		WillReturnResult(NewResult("INSERT", 1))

	_, err = mock.Exec(context.Background(), "INSERT INTO mytable(a, b) VALUES (?, ?)", "A", "B")
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// test the case when db is not triggered and expectations
// are not asserted on close
func TestIssue4(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectQuery("some sql query which will not be called").
		WillReturnRows(NewRows([]string{"id"}))

	if err := mock.ExpectationsWereMet(); err == nil {
		t.Errorf("was expecting an error since query was not triggered")
	}
}

func TestMockQuery(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rs := NewRows([]string{"id", "title"}).AddRow(5, "hello world")

	mock.ExpectQuery("SELECT (.+) FROM articles WHERE id = ?").
		WithArgs(5).
		WillReturnRows(rs)

	rows, err := mock.Query(context.Background(), "SELECT (.+) FROM articles WHERE id = ?", 5)
	if err != nil {
		t.Errorf("error '%s' was not expected while retrieving mock rows", err)
	}

	defer rows.Close()

	if !rows.Next() {
		t.Error("it must have had one row as result, but got empty result set instead")
	}

	var id int
	var title string

	err = rows.Scan(&id, &title)
	if err != nil {
		t.Errorf("error '%s' was not expected while trying to scan row", err)
	}

	if id != 5 {
		t.Errorf("expected mocked id to be 5, but got %d instead", id)
	}

	if title != "hello world" {
		t.Errorf("expected mocked title to be 'hello world', but got '%s' instead", title)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMockCopyFrom(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectCopyFrom(`"fooschema"."baztable"`, []string{"col1"}).
		WillReturnResult(2).WillDelayFor(1 * time.Second)

	_, err = mock.CopyFrom(context.Background(), pgx.Identifier{"error", "error"}, []string{"error"}, nil)
	if err == nil {
		t.Error("error is expected while executing CopyFrom")
	}
	if mock.ExpectationsWereMet() == nil {
		t.Error("there must be unfulfilled expectations")
	}

	rows, err := mock.CopyFrom(context.Background(), pgx.Identifier{"fooschema", "baztable"}, []string{"col1"}, nil)
	if err != nil {
		t.Errorf("error '%s' was not expected while executing CopyFrom", err)
	}

	if rows != 2 {
		t.Errorf("expected RowsAffected to be 2, but got %d instead", rows)
	}

	mock.ExpectCopyFrom(`"fooschema"."baztable"`, []string{"col1"}).
		WillReturnError(errors.New("error is here"))

	_, err = mock.CopyFrom(context.Background(), pgx.Identifier{"fooschema", "baztable"}, []string{"col1"}, nil)
	if err == nil {
		t.Error("error is expected while executing CopyFrom")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMockQueryTypes(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	columns := []string{"id", "timestamp", "sold"}

	timestamp := time.Now()
	rs := NewRows(columns)
	rs.AddRow(5, timestamp, true)

	mock.ExpectQuery("SELECT (.+) FROM sales WHERE id = ?").
		WithArgs(5).
		WillReturnRows(rs)

	rows, err := mock.Query(context.Background(), "SELECT (.+) FROM sales WHERE id = ?", 5)
	if err != nil {
		t.Errorf("error '%s' was not expected while retrieving mock rows", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Error("it must have had one row as result, but got empty result set instead")
	}

	var id int
	var time time.Time
	var sold bool

	err = rows.Scan(&id, &time, &sold)
	if err != nil {
		t.Errorf("error '%s' was not expected while trying to scan row", err)
	}

	if id != 5 {
		t.Errorf("expected mocked id to be 5, but got %d instead", id)
	}

	if time != timestamp {
		t.Errorf("expected mocked time to be %s, but got '%s' instead", timestamp, time)
	}

	if sold != true {
		t.Errorf("expected mocked boolean to be true, but got %v instead", sold)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestTransactionExpectations(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	// begin and commit
	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, err := mock.Begin(context.Background())
	if err != nil {
		t.Errorf("an error '%s' was not expected when beginning a transaction", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		t.Errorf("an error '%s' was not expected when committing a transaction", err)
	}

	// beginTx and commit
	mock.ExpectBeginTx(pgx.TxOptions{})
	mock.ExpectCommit()

	tx, err = mock.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Errorf("an error '%s' was not expected when beginning a transaction", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		t.Errorf("an error '%s' was not expected when committing a transaction", err)
	}

	// begin and rollback
	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err = mock.Begin(context.Background())
	if err != nil {
		t.Errorf("an error '%s' was not expected when beginning a transaction", err)
	}

	err = tx.Rollback(context.Background())
	if err != nil {
		t.Errorf("an error '%s' was not expected when rolling back a transaction", err)
	}

	// begin with an error
	mock.ExpectBegin().WillReturnError(fmt.Errorf("some err"))

	_, err = mock.Begin(context.Background())
	if err == nil {
		t.Error("an error was expected when beginning a transaction, but got none")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPrepareExpectations(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectPrepare("foo", "SELECT (.+) FROM articles WHERE id = ?").
		WillDelayFor(1 * time.Second).
		WillReturnCloseError(errors.New("invaders must die"))

	stmt, err := mock.Prepare(context.Background(), "foo", "SELECT (.+) FROM articles WHERE id = $1")
	if err != nil {
		t.Errorf("error '%s' was not expected while creating a prepared statement", err)
	}
	if stmt == nil {
		t.Errorf("stmt was expected while creating a prepared statement")
	}

	// expect something else, w/o ExpectPrepare()
	var id int
	var title string
	rs := NewRows([]string{"id", "title"}).AddRow(5, "hello world")

	mock.ExpectQuery("foo").
		WithArgs(5).
		WillReturnRows(rs)

	err = mock.QueryRow(context.Background(), "foo", 5).Scan(&id, &title)
	if err != nil {
		t.Errorf("error '%s' was not expected while retrieving mock rows", err)
	}

	mock.ExpectPrepare("foo", "SELECT (.+) FROM articles WHERE id = ?").
		WillReturnError(fmt.Errorf("Some DB error occurred"))

	stmt, err = mock.Prepare(context.Background(), "foo", "SELECT id FROM articles WHERE id = $1")
	if err == nil {
		t.Error("error was expected while creating a prepared statement")
	}
	if stmt != nil {
		t.Errorf("stmt was not expected while creating a prepared statement returning error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPreparedQueryExecutions(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectPrepare("foo", "SELECT (.+) FROM articles WHERE id = ?")

	rs1 := NewRows([]string{"id", "title"}).AddRow(5, "hello world")
	mock.ExpectQuery("foo").
		WithArgs(5).
		WillReturnRows(rs1)

	rs2 := NewRows([]string{"id", "title"}).AddRow(2, "whoop")
	mock.ExpectQuery("foo").
		WithArgs(2).
		WillReturnRows(rs2)

	_, err = mock.Prepare(context.Background(), "foo", "SELECT id, title FROM articles WHERE id = ?")
	if err != nil {
		t.Errorf("error '%s' was not expected while creating a prepared statement", err)
	}

	var id int
	var title string
	err = mock.QueryRow(context.Background(), "foo", 5).Scan(&id, &title)
	if err != nil {
		t.Errorf("error '%s' was not expected querying row from statement and scanning", err)
	}

	if id != 5 {
		t.Errorf("expected mocked id to be 5, but got %d instead", id)
	}

	if title != "hello world" {
		t.Errorf("expected mocked title to be 'hello world', but got '%s' instead", title)
	}

	err = mock.QueryRow(context.Background(), "foo", 2).Scan(&id, &title)
	if err != nil {
		t.Errorf("error '%s' was not expected querying row from statement and scanning", err)
	}

	if id != 2 {
		t.Errorf("expected mocked id to be 2, but got %d instead", id)
	}

	if title != "whoop" {
		t.Errorf("expected mocked title to be 'whoop', but got '%s' instead", title)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestUnorderedPreparedQueryExecutions(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.MatchExpectationsInOrder(false)

	mock.ExpectPrepare("articles_stmt", "SELECT (.+) FROM articles WHERE id = ?").
		ExpectQuery().
		WithArgs(5).
		WillReturnRows(NewRows([]string{"id", "title"}).AddRow(5, "The quick brown fox"))
	mock.ExpectPrepare("authors_stmt", "SELECT (.+) FROM authors WHERE id = ?").
		ExpectQuery().
		WithArgs(1).
		WillReturnRows(NewRows([]string{"id", "title"}).AddRow(1, "Betty B."))

	var id int
	var name string

	_, err = mock.Prepare(context.Background(), "authors_stmt", "SELECT id, name FROM authors WHERE id = ?")
	if err != nil {
		t.Errorf("error '%s' was not expected while creating a prepared statement", err)
	}

	err = mock.QueryRow(context.Background(), "authors_stmt", 1).Scan(&id, &name)
	if err != nil {
		t.Errorf("error '%s' was not expected querying row from statement and scanning", err)
	}

	if name != "Betty B." {
		t.Errorf("expected mocked name to be 'Betty B.', but got '%s' instead", name)
	}
}

func TestUnexpectedOperations(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectPrepare("foo", "SELECT (.+) FROM articles WHERE id = ?")
	_, err = mock.Prepare(context.Background(), "foo", "SELECT id, title FROM articles WHERE id = ?")
	if err != nil {
		t.Errorf("error '%s' was not expected while creating a prepared statement", err)
	}

	var id int
	var title string

	err = mock.QueryRow(context.Background(), "foo", 5).Scan(&id, &title)
	if err == nil {
		t.Error("error was expected querying row, since there was no such expectation")
	}

	mock.ExpectRollback()

	if err := mock.ExpectationsWereMet(); err == nil {
		t.Errorf("was expecting an error since query was not triggered")
	}
}

func TestWrongExpectations(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectBegin()

	rs1 := NewRows([]string{"id", "title"}).AddRow(5, "hello world")
	mock.ExpectQuery("SELECT (.+) FROM articles WHERE id = ?").
		WithArgs(5).
		WillReturnRows(rs1)

	mock.ExpectCommit().WillReturnError(fmt.Errorf("deadlock occurred"))
	mock.ExpectRollback() // won't be triggered

	var id int
	var title string

	err = mock.QueryRow(context.Background(), "SELECT id, title FROM articles WHERE id = ? FOR UPDATE", 5).Scan(&id, &title)
	if err == nil {
		t.Error("error was expected while querying row, since there begin transaction expectation is not fulfilled")
	}

	// lets go around and start transaction
	tx, err := mock.Begin(context.Background())
	if err != nil {
		t.Errorf("an error '%s' was not expected when beginning a transaction", err)
	}

	err = mock.QueryRow(context.Background(), "SELECT id, title FROM articles WHERE id = ? FOR UPDATE", 5).Scan(&id, &title)
	if err != nil {
		t.Errorf("error '%s' was not expected while querying row, since transaction was started", err)
	}

	err = tx.Commit(context.Background())
	if err == nil {
		t.Error("a deadlock error was expected when committing a transaction", err)
	}

	if err := mock.ExpectationsWereMet(); err == nil {
		t.Errorf("was expecting an error since query was not triggered")
	}
}

func TestExecExpectations(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	result := NewResult("INSERT", 1)
	mock.ExpectExec("^INSERT INTO articles").
		WithArgs("hello").
		WillReturnResult(result)

	res, err := mock.Exec(context.Background(), "INSERT INTO articles (title) VALUES (?)", "hello")
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if res.RowsAffected() != 1 {
		t.Errorf("expected affected rows to be 1, but got %d instead", res.RowsAffected())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestRowBuilderAndNilTypes(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rs := NewRows([]string{"id", "active", "created", "status"}).
		AddRow(1, true, NullTime{time.Now(), true}, NullInt{5, true}).
		AddRow(2, false, NullTime{Valid: false}, NullInt{Valid: false})

	mock.ExpectQuery("SELECT (.+) FROM sales").WillReturnRows(rs)

	rows, err := mock.Query(context.Background(), "SELECT * FROM sales")
	if err != nil {
		t.Errorf("error '%s' was not expected while retrieving mock rows", err)
	}
	defer rows.Close()

	// NullTime and NullInt are used from stubs_test.go
	var (
		id      int
		active  bool
		created NullTime
		status  NullInt
	)

	if !rows.Next() {
		t.Error("it must have had row in rows, but got empty result set instead")
	}

	err = rows.Scan(&id, &active, &created, &status)
	if err != nil {
		t.Errorf("error '%s' was not expected while trying to scan row", err)
	}

	if id != 1 {
		t.Errorf("expected mocked id to be 1, but got %d instead", id)
	}

	if !active {
		t.Errorf("expected 'active' to be 'true', but got '%v' instead", active)
	}

	if !created.Valid {
		t.Errorf("expected 'created' to be valid, but it %+v is not", created)
	}

	if !status.Valid {
		t.Errorf("expected 'status' to be valid, but it %+v is not", status)
	}

	if status.Integer != 5 {
		t.Errorf("expected 'status' to be '5', but got '%d'", status.Integer)
	}

	// test second row
	if !rows.Next() {
		t.Error("it must have had row in rows, but got empty result set instead")
	}

	err = rows.Scan(&id, &active, &created, &status)
	if err != nil {
		t.Errorf("error '%s' was not expected while trying to scan row", err)
	}

	if id != 2 {
		t.Errorf("expected mocked id to be 2, but got %d instead", id)
	}

	if active {
		t.Errorf("expected 'active' to be 'false', but got '%v' instead", active)
	}

	if created.Valid {
		t.Errorf("expected 'created' to be invalid, but it %+v is not", created)
	}

	if status.Valid {
		t.Errorf("expected 'status' to be invalid, but it %+v is not", status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestArgumentReflectValueTypeError(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rs := NewRows([]string{"id"}).AddRow(1)

	mock.ExpectQuery("SELECT (.+) FROM sales").WithArgs(5.5).WillReturnRows(rs)

	_, err = mock.Query(context.Background(), "SELECT * FROM sales WHERE x = ?", 5)
	if err == nil {
		t.Error("expected error, but got none")
	}
}

func TestGoroutineExecutionWithUnorderedExpectationMatching(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	// note this line is important for unordered expectation matching
	mock.MatchExpectationsInOrder(false)

	result := NewResult("UPDATE", 1)

	mock.ExpectExec("^UPDATE one").WithArgs("one").WillReturnResult(result)
	mock.ExpectExec("^UPDATE two").WithArgs("one", "two").WillReturnResult(result)
	mock.ExpectExec("^UPDATE three").WithArgs("one", "two", "three").WillReturnResult(result)

	var wg sync.WaitGroup
	queries := map[string][]interface{}{
		"one":   {"one"},
		"two":   {"one", "two"},
		"three": {"one", "two", "three"},
	}

	wg.Add(len(queries))
	for table, args := range queries {
		go func(tbl string, a []interface{}) {
			if _, err := mock.Exec(context.Background(), "UPDATE "+tbl, a...); err != nil {
				t.Errorf("error was not expected: %s", err)
			}
			wg.Done()
		}(table, args)
	}

	wg.Wait()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// func Test_goroutines() {
// 	mock, err := NewConn()
// 	if err != nil {
// 		fmt.Println("failed to open pgxmock database:", err)
// 	}
// 	defer mock.Close(context.Background())

// 	// note this line is important for unordered expectation matching
// 	mock.MatchExpectationsInOrder(false)

// 	result := NewResult("UPDATE", 1)

// 	mock.ExpectExec("^UPDATE one").WithArgs("one").WillReturnResult(result)
// 	mock.ExpectExec("^UPDATE two").WithArgs("one", "two").WillReturnResult(result)
// 	mock.ExpectExec("^UPDATE three").WithArgs("one", "two", "three").WillReturnResult(result)

// 	var wg sync.WaitGroup
// 	queries := map[string][]interface{}{
// 		"one":   {"one"},
// 		"two":   {"one", "two"},
// 		"three": {"one", "two", "three"},
// 	}

// 	wg.Add(len(queries))
// 	for table, args := range queries {
// 		go func(tbl string, a []interface{}) {
// 			if _, err := mock.Exec(context.Background(), "UPDATE "+tbl, a...); err != nil {
// 				fmt.Println("error was not expected:", err)
// 			}
// 			wg.Done()
// 		}(table, args)
// 	}

// 	wg.Wait()

// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		fmt.Println("there were unfulfilled expectations:", err)
// 	}
// 	// Output:
// }

// False Positive - passes despite mismatched Exec
// see #37 issue
func TestRunExecsWithOrderedShouldNotMeetAllExpectations(t *testing.T) {
	dbmock, _ := NewConn()
	dbmock.ExpectExec("THE FIRST EXEC")
	dbmock.ExpectExec("THE SECOND EXEC")

	_, _ = dbmock.Exec(context.Background(), "THE FIRST EXEC")
	_, _ = dbmock.Exec(context.Background(), "THE WRONG EXEC")

	err := dbmock.ExpectationsWereMet()
	if err == nil {
		t.Fatal("was expecting an error, but there wasn't any")
	}
}

// False Positive - passes despite mismatched Exec
// see #37 issue
func TestRunQueriesWithOrderedShouldNotMeetAllExpectations(t *testing.T) {
	dbmock, _ := NewConn()
	dbmock.ExpectQuery("THE FIRST QUERY")
	dbmock.ExpectQuery("THE SECOND QUERY")

	_, _ = dbmock.Query(context.Background(), "THE FIRST QUERY")
	_, _ = dbmock.Query(context.Background(), "THE WRONG QUERY")

	err := dbmock.ExpectationsWereMet()
	if err == nil {
		t.Fatal("was expecting an error, but there wasn't any")
	}
}

func TestRunExecsWithExpectedErrorMeetsExpectations(t *testing.T) {
	dbmock, _ := NewConn()
	dbmock.ExpectExec("THE FIRST EXEC").WillReturnError(fmt.Errorf("big bad bug"))
	dbmock.ExpectExec("THE SECOND EXEC").WillReturnResult(NewResult("UPDATE", 0))

	_, _ = dbmock.Exec(context.Background(), "THE FIRST EXEC")
	_, _ = dbmock.Exec(context.Background(), "THE SECOND EXEC")

	err := dbmock.ExpectationsWereMet()
	if err != nil {
		t.Fatalf("all expectations should be met: %s", err)
	}
}

func TestRunQueryWithExpectedErrorMeetsExpectations(t *testing.T) {
	dbmock, _ := NewConn()
	dbmock.ExpectQuery("THE FIRST QUERY").WillReturnError(fmt.Errorf("big bad bug"))
	dbmock.ExpectQuery("THE SECOND QUERY").WillReturnRows(NewRows([]string{"col"}).AddRow(1))

	_, _ = dbmock.Query(context.Background(), "THE FIRST QUERY")
	_, _ = dbmock.Query(context.Background(), "THE SECOND QUERY")

	err := dbmock.ExpectationsWereMet()
	if err != nil {
		t.Fatalf("all expectations should be met: %s", err)
	}
}

func TestEmptyRowSet(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rs := NewRows([]string{"id", "title"})

	mock.ExpectQuery("SELECT (.+) FROM articles WHERE id = ?").
		WithArgs(5).
		WillReturnRows(rs)

	rows, err := mock.Query(context.Background(), "SELECT (.+) FROM articles WHERE id = ?", 5)
	if err != nil {
		t.Errorf("error '%s' was not expected while retrieving mock rows", err)
	}
	defer rows.Close()

	if rows.Next() {
		t.Error("expected no rows but got one")
	}

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Fatalf("all expectations should be met: %s", err)
	}
}

// Based on issue #50
func TestPrepareExpectationNotFulfilled(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectPrepare("foo", "^BADSELECT$")

	if _, err := mock.Prepare(context.Background(), "foo", "SELECT"); err == nil {
		t.Fatal("prepare should not match expected query string")
	}

	if err := mock.ExpectationsWereMet(); err == nil {
		t.Errorf("was expecting an error, since prepared statement query does not match, but there was none")
	}
}

func TestRollbackThrow(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	// columns to be used for result
	columns := []string{"id", "status"}
	// expect transaction begin
	mock.ExpectBegin()
	// expect query to fetch order, match it with regexp
	mock.ExpectQuery("SELECT (.+) FROM orders (.+) FOR UPDATE").
		WithArgs(1).
		WillReturnRows(NewRows(columns).AddRow(1, 1))
	// expect transaction rollback, since order status is "cancelled"
	mock.ExpectRollback().WillReturnError(fmt.Errorf("rollback failed"))

	// run the cancel order function
	someOrderID := 1
	// call a function which executes expected database operations
	err = cancelOrder(mock, someOrderID)
	if err == nil {
		t.Error("an error was expected when rolling back transaction, but got none")
	}

	// ensure all expectations have been met
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectation error: %s", err)
	}
	// Output:
}

func TestUnexpectedBegin(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	if _, err := mock.Begin(context.Background()); err == nil {
		t.Error("an error was expected when calling begin, but got none")
	}
}

func TestUnexpectedExec(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectBegin()
	_, _ = mock.Begin(context.Background())
	if _, err := mock.Exec(context.Background(), "SELECT 1"); err == nil {
		t.Error("an error was expected when calling exec, but got none")
	}
}

func TestUnexpectedCommit(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectBegin()
	tx, _ := mock.Begin(context.Background())
	if err := tx.Commit(context.Background()); err == nil {
		t.Error("an error was expected when calling commit, but got none")
	}
}

func TestUnexpectedCommitOrder(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(fmt.Errorf("Rollback failed"))
	tx, _ := mock.Begin(context.Background())
	if err := tx.Commit(context.Background()); err == nil {
		t.Error("an error was expected when calling commit, but got none")
	}
}

func TestExpectedCommitOrder(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectCommit().WillReturnError(fmt.Errorf("Commit failed"))
	if _, err := mock.Begin(context.Background()); err == nil {
		t.Error("an error was expected when calling begin, but got none")
	}
}

func TestUnexpectedRollback(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectBegin()
	tx, _ := mock.Begin(context.Background())
	if err := tx.Rollback(context.Background()); err == nil {
		t.Error("an error was expected when calling rollback, but got none")
	}
}

func TestUnexpectedRollbackOrder(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectBegin()

	tx, _ := mock.Begin(context.Background())
	if err := tx.Rollback(context.Background()); err == nil {
		t.Error("an error was expected when calling rollback, but got none")
	}
}

func TestPrepareExec(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	defer mock.Close(context.Background())
	mock.ExpectBegin()
	ep := mock.ExpectPrepare("foo", "INSERT INTO ORDERS\\(ID, STATUS\\) VALUES \\(\\?, \\?\\)")
	for i := 0; i < 3; i++ {
		ep.ExpectExec().WithArgs(AnyArg(), AnyArg()).WillReturnResult(NewResult("UPDATE", 1))
	}
	mock.ExpectCommit()
	tx, _ := mock.Begin(context.Background())
	_, err = tx.Prepare(context.Background(), "foo", "INSERT INTO ORDERS(ID, STATUS) VALUES (?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, err := mock.Exec(context.Background(), "foo", i, "Hello"+strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = tx.Commit(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPrepareQuery(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	defer mock.Close(context.Background())
	mock.ExpectBegin()
	ep := mock.ExpectPrepare("foo", "SELECT ID, STATUS FROM ORDERS WHERE ID = \\?")
	ep.ExpectQuery().WithArgs(101).WillReturnRows(NewRows([]string{"ID", "STATUS"}).AddRow(101, "Hello"))
	mock.ExpectCommit()
	tx, _ := mock.Begin(context.Background())
	_, err = tx.Prepare(context.Background(), "foo", "SELECT ID, STATUS FROM ORDERS WHERE ID = ?")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := mock.Query(context.Background(), "foo", 101)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id     int
			status string
		)
		if _ = rows.Scan(&id, &status); id != 101 || status != "Hello" {
			t.Fatal("wrong query results")
		}

	}
	_ = tx.Commit(context.Background())
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestExpectedCloseError(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectClose().WillReturnError(fmt.Errorf("Close failed"))
	if err := mock.Close(context.Background()); err == nil {
		t.Error("an error was expected when calling close, but got none")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestExpectedCloseOrder(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	defer mock.Close(context.Background())
	mock.ExpectClose().WillReturnError(fmt.Errorf("Close failed"))
	_, _ = mock.Begin(context.Background())
	if err := mock.ExpectationsWereMet(); err == nil {
		t.Error("expected error on ExpectationsWereMet")
	}
}

func TestExpectedBeginOrder(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	mock.ExpectBegin().WillReturnError(fmt.Errorf("Begin failed"))
	if err := mock.Close(context.Background()); err == nil {
		t.Error("an error was expected when calling close, but got none")
	}
}

func TestPreparedStatementCloseExpectation(t *testing.T) {
	// Open new mock database
	mock, err := NewConn()
	if err != nil {
		fmt.Println("error creating mock database")
		return
	}
	defer mock.Close(context.Background())

	ep := mock.ExpectPrepare("foo", "INSERT INTO ORDERS").WillBeClosed()
	ep.ExpectExec().WithArgs(AnyArg(), AnyArg()).WillReturnResult(NewResult("UPDATE", 1))

	_, err = mock.Prepare(context.Background(), "foo", "INSERT INTO ORDERS(ID, STATUS) VALUES (?, ?)")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := mock.Exec(context.Background(), "foo", 1, "Hello"); err != nil {
		t.Fatal(err)
	}

	if err := mock.Deallocate(context.Background(), "foo"); err != nil {
		t.Fatal(err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestExecExpectationErrorDelay(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	// test that return of error is delayed
	delay := time.Millisecond * 100
	mock.ExpectExec("^INSERT INTO articles").WithArgs(AnyArg()).
		WillReturnError(errors.New("slow fail")).
		WillDelayFor(delay)

	start := time.Now()
	res, err := mock.Exec(context.Background(), "INSERT INTO articles (title) VALUES (?)", "hello")
	stop := time.Now()

	if res.String() != "" {
		t.Errorf("result was not expected, was expecting nil")
	}

	if err == nil {
		t.Errorf("error was expected, was not expecting nil")
	}

	if err.Error() != "slow fail" {
		t.Errorf("error '%s' was not expected, was expecting '%s'", err.Error(), "slow fail")
	}

	elapsed := stop.Sub(start)
	if elapsed < delay {
		t.Errorf("expecting a delay of %v before error, actual delay was %v", delay, elapsed)
	}

	// also test that return of error is not delayed
	mock.ExpectExec("^INSERT INTO articles").WillReturnError(errors.New("fast fail"))

	start = time.Now()
	_, _ = mock.Exec(context.Background(), "INSERT INTO articles (title) VALUES (?)", "hello")
	stop = time.Now()

	elapsed = stop.Sub(start)
	if elapsed > delay {
		t.Errorf("expecting a delay of less than %v before error, actual delay was %v", delay, elapsed)
	}
}

func TestOptionsFail(t *testing.T) {
	t.Parallel()
	expected := errors.New("failing option")
	option := func(*pgxmock) error {
		return expected
	}
	mock, err := NewConn(option)
	defer func() { _ = mock.Close(context.Background()) }()
	if err == nil {
		t.Errorf("missing expecting error '%s' when opening a stub database connection", expected)
	}
}

func TestNewRows(t *testing.T) {
	t.Parallel()
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())
	columns := []string{"col1", "col2"}

	r := mock.NewRows(columns)
	if len(r.defs) != len(columns) || string(r.defs[0].Name) != columns[0] || string(r.defs[1].Name) != columns[1] {
		t.Errorf("expecting to create a row with columns %v, actual colmns are %v", r.defs, columns)
	}
}

// This is actually a test of ExpectationsWereMet. Without a lock around e.fulfilled() inside
// ExpectationWereMet, the race detector complains if e.triggered is being read while it is also
// being written by the query running in another goroutine.
func TestQueryWithTimeout(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	rs := NewRows([]string{"id", "title"}).FromCSVString("5,hello world")

	mock.ExpectQuery("SELECT (.+) FROM articles WHERE id = ?").
		WillDelayFor(50 * time.Millisecond). // Query will take longer than timeout
		WithArgs(5).
		WillReturnRows(rs)

	_, err = queryWithTimeout(10*time.Millisecond, mock, "SELECT (.+) FROM articles WHERE id = ?", 5)
	if err == nil {
		t.Errorf("expecting query to time out")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func queryWithTimeout(t time.Duration, db pgxIface, query string, args ...interface{}) (pgx.Rows, error) {
	rowsChan := make(chan pgx.Rows, 1)
	errChan := make(chan error, 1)

	go func() {
		rows, err := db.Query(context.Background(), query, args...)
		if err != nil {
			errChan <- err
			return
		}
		rowsChan <- rows
	}()

	select {
	case rows := <-rowsChan:
		return rows, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(t):
		return nil, fmt.Errorf("query timed out after %v", t)
	}
}

func TestCon(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The Conn() did not panic")
		}
	}()
	_ = mock.Conn()
}

func TestConnInfo(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	_ = mock.Config()
}

func TestPgConn(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	_ = mock.PgConn()
}

func TestNewRowsWithColumnDefinition(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())
	r := mock.NewRowsWithColumnDefinition(*mock.NewColumn("foo"))
	if len(r.defs) != 1 {
		t.Error("NewRows failed")
	}
}
