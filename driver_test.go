package pgxmock

import (
	"testing"
)

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
