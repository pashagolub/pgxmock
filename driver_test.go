package pgxmock

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

type void struct{}

func (void) Print(...interface{}) {}

type converter struct{}

func (c *converter) ConvertValue(v interface{}) (driver.Value, error) {
	return nil, errors.New("converter disabled")
}

func TestShouldOpenConnectionIssue15(t *testing.T) {
	mock, err := New()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	if len(pool.conns) != 1 {
		t.Errorf("expected 1 connection in pool, but there is: %d", len(pool.conns))
	}

	smock, _ := mock.(*pgxmock)
	if smock.opened != 1 {
		t.Errorf("expected 1 connection on mock to be opened, but there is: %d", smock.opened)
	}

	// defer so the rows gets closed first
	defer func() {
		if smock.opened != 0 {
			t.Errorf("expected no connections on mock to be opened, but there is: %d", smock.opened)
		}
	}()

	mock.ExpectQuery("SELECT").WillReturnRows(NewRows([]string{"one", "two"}).AddRow("val1", "val2"))
	rows, err := mock.Query(context.Background(), "SELECT")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	defer rows.Close()

	mock.ExpectExec("UPDATE").WillReturnResult(NewResult("UPDATE", 1))
	if _, err = mock.Exec(context.Background(), "UPDATE"); err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	// now there should be two connections open
	if smock.opened != 2 {
		t.Errorf("expected 2 connection on mock to be opened, but there is: %d", smock.opened)
	}

	mock.ExpectClose()
	if err = mock.Close(); err != nil {
		t.Errorf("expected no error on close, but got: %s", err)
	}

	// one is still reserved for rows
	if smock.opened != 1 {
		t.Errorf("expected 1 connection on mock to be still reserved for rows, but there is: %d", smock.opened)
	}
}

func TestTwoOpenConnectionsOnTheSameDSN(t *testing.T) {
	mock, err := New()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	mock2, err := New()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	if len(pool.conns) != 2 {
		t.Errorf("expected 2 connection in pool, but there is: %d", len(pool.conns))
	}

	if mock == mock2 {
		t.Errorf("expected not the same database instance, but it is the same")
	}
	if mock == mock2 {
		t.Errorf("expected not the same mock instance, but it is the same")
	}
}

// func TestWithOptions(t *testing.T) {
// 	c := &converter{}
// 	_, mock, err := New(ValueConverterOption(c))
// 	if err != nil {
// 		t.Errorf("expected no error, but got: %s", err)
// 	}
// 	smock, _ := mock.(*pgxmock)
// 	if smock.converter.(*converter) != c {
// 		t.Errorf("expected a custom converter to be set")
// 	}
// }
