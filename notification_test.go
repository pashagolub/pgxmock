package pgxmock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestWaitForNotification(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	want := &pgconn.Notification{PID: 100, Channel: "chat", Payload: "hello"}
	mock.ExpectWaitForNotification().WillReturnNotification(want)

	got, err := mock.WaitForNotification(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// A LISTEN/NOTIFY loop typically issues LISTEN and then waits repeatedly.
func TestWaitForNotificationLoop(t *testing.T) {
	mock, err := NewConn(QueryMatcherOption(QueryMatcherEqual))
	assert.NoError(t, err)

	mock.ExpectExec("listen chat").WillReturnResult(NewResult("LISTEN", 0))
	for _, payload := range []string{"first", "second"} {
		mock.ExpectWaitForNotification().
			WillReturnNotification(&pgconn.Notification{Channel: "chat", Payload: payload})
	}

	_, err = mock.Exec(context.Background(), "listen chat")
	assert.NoError(t, err)

	var got []string
	for range 2 {
		n, err := mock.WaitForNotification(context.Background())
		assert.NoError(t, err)
		got = append(got, n.Payload)
	}
	assert.Equal(t, []string{"first", "second"}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWaitForNotificationError(t *testing.T) {
	errListen := errors.New("connection lost while listening")
	mock, err := NewConn()
	assert.NoError(t, err)
	mock.ExpectWaitForNotification().WillReturnError(errListen)

	n, err := mock.WaitForNotification(context.Background())
	assert.Nil(t, n)
	assert.ErrorIs(t, err, errListen)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The usual pattern is to wait with a deadline; a wait that outlives it must
// report the context error rather than a notification.
func TestWaitForNotificationContextTimeout(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)
	mock.ExpectWaitForNotification().
		WillReturnNotification(&pgconn.Notification{Channel: "chat"}).
		WillDelayFor(time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	n, err := mock.WaitForNotification(ctx)
	assert.Nil(t, n)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitForNotificationNotExpected(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)

	n, err := mock.WaitForNotification(context.Background())
	assert.Nil(t, n)
	assert.ErrorContains(t, err, "call to method WaitForNotification() was not expected")
}

func TestWaitForNotificationString(t *testing.T) {
	mock, err := NewConn()
	assert.NoError(t, err)
	e := mock.ExpectWaitForNotification().
		WillReturnNotification(&pgconn.Notification{Channel: "chat", Payload: "hello"})

	assert.Contains(t, e.String(), "WaitForNotification()")
	assert.Contains(t, e.String(), "chat")
	assert.Contains(t, e.String(), "hello")
}
