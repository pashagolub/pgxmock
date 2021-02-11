package pgxmock

import (
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

func TestTwoOpenConnectionsOnTheSameDSN(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	mock2, err := NewConn()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	if mock == mock2 {
		t.Errorf("expected not the same mock instance, but it is the same")
	}
}
