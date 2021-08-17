package pgxmock

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func ExampleExpectedExec() {
	mock, _ := NewConn()
	result := NewErrorResult(fmt.Errorf("some error"))
	mock.ExpectExec("^INSERT (.+)").WillReturnResult(result)
	res, _ := mock.Exec(context.Background(), "INSERT something")
	s := res.String()
	fmt.Println(s)
	// Output: some error
}

func TestUnmonitoredPing(t *testing.T) {
	mock, _ := NewConn()
	p := mock.ExpectPing()
	if p != nil {
		t.Error("ExpectPing should return nil since MonitorPingsOption = false ")
	}
}

func TestBuildQuery(t *testing.T) {
	mock, _ := NewConn(MonitorPingsOption(true))
	query := `
		SELECT
			name,
			email,
			address,
			anotherfield
		FROM user
		where
			name    = 'John'
			and
			address = 'Jakarta'

	`

	mock.ExpectPing()
	mock.ExpectQuery(query)
	mock.ExpectExec(query)
	mock.ExpectPrepare("foo", query)

	_ = mock.Ping(context.Background())
	mock.QueryRow(context.Background(), query)
	_, _ = mock.Exec(context.Background(), query)
	_, _ = mock.Prepare(context.Background(), "foo", query)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestQueryRowScan(t *testing.T) {
	mock, _ := NewConn() //TODO New(ValueConverterOption(CustomConverter{}))
	query := `
		SELECT
			name,
			email,
			address,
			anotherfield
		FROM user
		where
			name    = 'John'
			and
			address = 'Jakarta'

	`
	expectedStringValue := "ValueOne"
	expectedIntValue := 2
	expectedArrayValue := []string{"Three", "Four"}
	mock.ExpectQuery(query).WillReturnRows(mock.NewRows([]string{"One", "Two", "Three"}).AddRow(expectedStringValue, expectedIntValue, []string{"Three", "Four"}))
	row := mock.QueryRow(context.Background(), query)
	var stringValue string
	var intValue int
	var arrayValue []string
	if e := row.Scan(&stringValue, &intValue, &arrayValue); e != nil {
		t.Error(e)
	}
	if stringValue != expectedStringValue {
		t.Errorf("Expectation %s does not met: %s", expectedStringValue, stringValue)
	}
	if intValue != expectedIntValue {
		t.Errorf("Expectation %d does not met: %d", expectedIntValue, intValue)
	}
	if !reflect.DeepEqual(expectedArrayValue, arrayValue) {
		t.Errorf("Expectation %v does not met: %v", expectedArrayValue, arrayValue)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
