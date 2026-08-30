package pgxmock

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgxmockConn struct {
	pgxmock
}

// NewConn creates PgxConnIface database connection and a mock to manage expectations.
// Accepts options, like QueryMatcherOption, to match SQL query strings in more sophisticated ways.
func NewConn(options ...func(*pgxmock) error) (PgxConnIface, error) {
	smock := &pgxmockConn{}
	smock.ordered = true
	return smock, smock.open(options)
}

func (c *pgxmockConn) Config() *pgx.ConnConfig {
	return &pgx.ConnConfig{}
}

type pgxmockPool struct {
	pgxmock
}

// NewPool creates PgxPoolIface pool of database connections and a mock to manage expectations.
// Accepts options, like QueryMatcherOption, to match SQL query strings in more sophisticated ways.
func NewPool(options ...func(*pgxmock) error) (PgxPoolIface, error) {
	smock := &pgxmockPool{}
	smock.ordered = true
	return smock, smock.open(options)
}

func (p *pgxmockPool) Close() {
	p.pgxmock.Close(context.Background())
}

// ErrAcquireNotSupported is returned by the pool methods that would have to
// hand out a *pgxpool.Conn. That is a concrete type whose internals cannot be
// constructed outside of pgxpool, so it cannot be mocked. Use
// PgxPoolIface.AsConn to get a mock connection instead.
var ErrAcquireNotSupported = errors.New("pgxmock: handing out a *pgxpool.Conn is not supported, use PgxPoolIface.AsConn() instead")

// Acquire is not supported, see ErrAcquireNotSupported.
func (p *pgxmockPool) Acquire(context.Context) (*pgxpool.Conn, error) {
	return nil, ErrAcquireNotSupported
}

// AcquireFunc is not supported, see ErrAcquireNotSupported. It reports the
// error instead of invoking f, so that a test cannot pass while the callback
// it was meant to exercise never ran.
func (p *pgxmockPool) AcquireFunc(context.Context, func(*pgxpool.Conn) error) error {
	return ErrAcquireNotSupported
}

// AcquireAllIdle always reports that the pool holds no idle connections.
func (p *pgxmockPool) AcquireAllIdle(context.Context) []*pgxpool.Conn {
	return []*pgxpool.Conn{}
}

func (p *pgxmockPool) Config() *pgxpool.Config {
	return &pgxpool.Config{ConnConfig: &pgx.ConnConfig{}}
}

// AsConn is similar to Acquire but returns proper mocking interface
func (p *pgxmockPool) AsConn() PgxConnIface {
	return &pgxmockConn{pgxmock: p.pgxmock}
}

func (p *pgxmockPool) Stat() *pgxpool.Stat {
	return &pgxpool.Stat{}
}
