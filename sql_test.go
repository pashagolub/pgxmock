package pgxmock_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v2"
)

func TestScanTime(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		panic(err)
	}

	now, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z07:00")

	mock.ExpectQuery(`SELECT now()`).
		WillReturnRows(
			mock.NewRows([]string{"stamp"}).
				AddRow(now))

	var value sql.NullTime
	err = mock.QueryRow(context.Background(), `SELECT now()`).Scan(&value)
	if err != nil {
		t.Error(err)
	}
	if value.Time != now {
		t.Errorf("want %v, got %v", now, value.Time)
	}
}
