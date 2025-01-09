package pgxmock

import (
	"context"
	"testing"
)

func TestTwoOpenConnectionsOnTheSameDSN(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
	}
	mock2, err := NewConn()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
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
		t.Fatalf("expected no error, but got: %s", err)
	}
	mock2, err := NewPool()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
	}
	if mock == mock2 {
		t.Errorf("expected not the same mock instance, but it is the same")
	}
	conn := mock.AsConn()
	if conn == nil {
		t.Error("expected connection strruct, but got nil")
	}
	mock.Close()
	mock2.Close()
}

func TestAcquire(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
	}
	_, err = mock.Acquire(context.Background())
	if err == nil {
		t.Error("expected error, but got nil")
	}
}

func TestPoolStat(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
	}
	s := mock.Stat()
	if s == nil {
		t.Error("expected stat object, but got nil")
	}
}

func TestPoolConfig(t *testing.T) {
	mock, err := NewPool()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
	}
	c := mock.Config()
	if c == nil {
		t.Error("expected config object, but got nil")
	}
	if c.ConnConfig == nil {
		t.Error("expected conn config object, but got nil")
	}
}

func TestConnConfig(t *testing.T) {
	mock, err := NewConn()
	if err != nil {
		t.Fatalf("expected no error, but got: %s", err)
	}
	c := mock.Config()
	if c == nil {
		t.Error("expected config object, but got nil")
	}
}
