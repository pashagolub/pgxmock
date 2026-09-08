package pgxmock

import (
	"errors"

	pgx "github.com/jackc/pgx/v5"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
)

// QueryMatcherOption allows to customize SQL query matcher
// and match SQL query strings in more sophisticated ways.
// The default QueryMatcher is QueryMatcherRegexp.
func QueryMatcherOption(queryMatcher QueryMatcher) func(*pgxmock) error {
	return func(s *pgxmock) error {
		s.queryMatcher = queryMatcher
		return nil
	}
}

// ErrorOnClosedConnOption makes the mock reject database operations once the
// connection has been closed, by returning pgconn.ErrConnClosed the way pgx
// does. Without it a closed mock keeps serving expectations, so a test cannot
// notice that its code used a handle it had already given up.
//
// Test for it with errors.Is, since pgx may return it wrapped.
func ErrorOnClosedConnOption() func(*pgxmock) error {
	return func(s *pgxmock) error {
		s.errorOnClosedConn = true
		return nil
	}
}

// ConnConfigOption makes Config report cfg, otherwise it reports a zero
// configuration:
//
//	cfg, err := pgx.ParseConfig("postgres://user@localhost:5432/orders")
//	mock, err := pgxmock.NewConn(pgxmock.ConnConfigOption(cfg))
func ConnConfigOption(cfg *pgx.ConnConfig) func(*pgxmock) error {
	return func(s *pgxmock) error {
		if cfg == nil {
			return errors.New("pgxmock: ConnConfigOption requires a non-nil *pgx.ConnConfig")
		}
		s.connConfig = cfg
		return nil
	}
}

// PoolConfigOption makes Config report cfg, and AsConn().Config() report
// cfg.ConnConfig. It cannot be combined with a different ConnConfigOption:
//
//	cfg, err := pgxpool.ParseConfig("postgres://user@localhost:5432/orders?pool_max_conns=8")
//	mock, err := pgxmock.NewPool(pgxmock.PoolConfigOption(cfg))
func PoolConfigOption(cfg *pgxpool.Config) func(*pgxmock) error {
	return func(s *pgxmock) error {
		if cfg == nil {
			return errors.New("pgxmock: PoolConfigOption requires a non-nil *pgxpool.Config")
		}
		s.poolConfig = cfg
		return nil
	}
}
