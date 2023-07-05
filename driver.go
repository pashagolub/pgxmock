package pgxmock

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgxmockConn struct {
	pgxmock
}

// NewConn creates PgxConnIface database connection and a mock to manage expectations.
// Accepts options, like ValueConverterOption, to use a ValueConverter from
// a specific driver.
// Pings db so that all expectations could be
// asserted.
func NewConn(options ...func(*pgxmock) error) (PgxConnIface, error) {
	smock := &pgxmockConn{}
	smock.ordered = true
	return smock, smock.open(options)
}

func (c *pgxmockConn) Close(ctx context.Context) error {
	return c.close(ctx)
}

type pgxmockPool struct {
	pgxmock
}

// NewPool creates PgxPoolIface pool of database connections and a mock to manage expectations.
// Accepts options, like ValueConverterOption, to use a ValueConverter from
// a specific driver.
// Pings db so that all expectations could be
// asserted.
func NewPool(options ...func(*pgxmock) error) (PgxPoolIface, error) {
	smock := &pgxmockPool{}
	smock.ordered = true
	return smock, smock.open(options)
}

func (p *pgxmockPool) Close() {
	_ = p.close(context.Background())
}

func (p *pgxmockPool) Acquire(context.Context) (*pgxpool.Conn, error) {
	return nil, errors.New("pgpool.Acquire() method is not implemented")
}

// AsConn is similar to Acquire but returns proper mocking interface
func (p *pgxmockPool) AsConn() PgxConnIface {
	return &pgxmockConn{pgxmock: p.pgxmock}
}

func (p *pgxmockPool) Stat() *pgxpool.Stat {
	return &pgxpool.Stat{}
}
