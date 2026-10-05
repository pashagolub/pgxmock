package pgxmock_test

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v6"
)

// listener is what nextChatMessage needs from *pgx.Conn.
type listener interface {
	WaitForNotification(ctx context.Context) (*pgconn.Notification, error)
}

// nextChatMessage is the code under test.
func nextChatMessage(ctx context.Context, db listener) (string, error) {
	n, err := db.WaitForNotification(ctx)
	if err != nil {
		return "", err
	}
	return n.Payload, nil
}

func ExampleExpectedWaitForNotification_WillReturnNotification() {
	mock, _ := pgxmock.NewConn()
	mock.ExpectWaitForNotification().
		WillReturnNotification(&pgconn.Notification{Channel: "chat", Payload: "hello"})

	msg, err := nextChatMessage(context.Background(), mock)
	fmt.Println(msg, err)
	// Output: hello <nil>
}
