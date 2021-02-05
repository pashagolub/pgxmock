package pgxmock

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type CustomConverter struct{}

func (s CustomConverter) ConvertValue(v interface{}) (interface{}, error) {
	switch v.(type) {
	case string:
		return v.(string), nil
	case []string:
		return v.([]string), nil
	case int:
		return v.(int), nil
	default:
		return nil, errors.New(fmt.Sprintf("cannot convert %T with value %v", v, v))
	}
}

func ExampleExpectedExec() {
	mock, _ := New()
	result := NewErrorResult(fmt.Errorf("some error"))
	mock.ExpectExec("^INSERT (.+)").WillReturnResult(result)
	res, _ := mock.Exec(context.Background(), "INSERT something")
	s := res.String()
	fmt.Println(s)
	// Output: some error
}

func TestBuildQuery(t *testing.T) {
	mock, _ := New()
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

	mock.ExpectQuery(query)
	mock.ExpectExec(query)
	mock.ExpectPrepare(query)

	mock.QueryRow(context.Background(), query)
	mock.Exec(context.Background(), query)
	mock.Prepare(context.Background(), "foo", query)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCustomValueConverterQueryScan(t *testing.T) {
	mock, _ := New() // New(ValueConverterOption(CustomConverter{}))
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
