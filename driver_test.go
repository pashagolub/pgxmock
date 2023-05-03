package pgxmock

import (
	"context"
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
	mock.Close(context.Background())
	mock2.Close(context.Background())
}

func TestPools(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	mock2, err := NewPool()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	if mock == mock2 {
		t.Errorf("expected not the same mock instance, but it is the same")
	}
	mock.Close()
	mock2.Close()
}

func TestAcquire(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	_, err = mock.Acquire(context.Background())
	if err == nil {
		t.Error("expected error, but got nil")
	}
}

func TestPoolStat(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}
	_ = mock.Stat()
	if err == nil {
		t.Error("expected error, but got nil")
	}
}
