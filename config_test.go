package pgxmock

import (
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnConfigOption(t *testing.T) {
	t.Parallel()
	cfg, err := pgx.ParseConfig("postgres://bob@localhost:5432/orders")
	require.NoError(t, err)
	cfg.MaxProtocolMessageBodyLen = 1 << 20

	mock, err := NewConn(ConnConfigOption(cfg))
	require.NoError(t, err)

	got := mock.Config()
	assert.Equal(t, "orders", got.Database)
	assert.Equal(t, "bob", got.User)
	assert.Equal(t, 1<<20, got.MaxProtocolMessageBodyLen)
}

func TestPoolConfigOption(t *testing.T) {
	t.Parallel()
	cfg, err := pgxpool.ParseConfig(
		"postgres://bob@localhost:5432/orders?pool_max_conns=8&pool_ping_timeout=5s")
	require.NoError(t, err)

	mock, err := NewPool(PoolConfigOption(cfg))
	require.NoError(t, err)

	got := mock.Config()
	assert.Equal(t, int32(8), got.MaxConns)
	assert.Equal(t, 5*time.Second, got.PingTimeout)
	assert.Equal(t, "orders", got.ConnConfig.Database)

	// the connection the pool hands out reports the same configuration
	assert.Equal(t, "orders", mock.AsConn().Config().Database)
}

func TestPoolConfigOptionCarriesConnConfigOption(t *testing.T) {
	t.Parallel()
	connCfg, err := pgx.ParseConfig("postgres://bob@localhost:5432/orders")
	require.NoError(t, err)

	mock, err := NewPool(ConnConfigOption(connCfg))
	require.NoError(t, err)
	assert.Equal(t, "orders", mock.Config().ConnConfig.Database)
}

func TestConflictingConfigOptions(t *testing.T) {
	t.Parallel()
	connCfg, err := pgx.ParseConfig("postgres://bob@localhost:5432/orders")
	require.NoError(t, err)
	poolCfg, err := pgxpool.ParseConfig("postgres://alice@localhost:5432/invoices")
	require.NoError(t, err)

	_, err = NewPool(ConnConfigOption(connCfg), PoolConfigOption(poolCfg))
	assert.ErrorContains(t, err, "pass only one of them")
}

func TestNilConfigOptions(t *testing.T) {
	t.Parallel()
	_, err := NewConn(ConnConfigOption(nil))
	assert.ErrorContains(t, err, "non-nil *pgx.ConnConfig")

	_, err = NewPool(PoolConfigOption(nil))
	assert.ErrorContains(t, err, "non-nil *pgxpool.Config")
}

func TestConfigDefaultsAreUsable(t *testing.T) {
	t.Parallel()
	conn, err := NewConn()
	require.NoError(t, err)
	assert.NotNil(t, conn.Config())

	pool, err := NewPool()
	require.NoError(t, err)
	require.NotNil(t, pool.Config())
	assert.NotNil(t, pool.Config().ConnConfig)
}

func TestConfigIsCopied(t *testing.T) {
	t.Parallel()
	cfg, err := pgxpool.ParseConfig("postgres://bob@localhost:5432/orders?pool_max_conns=8")
	require.NoError(t, err)

	mock, err := NewPool(PoolConfigOption(cfg))
	require.NoError(t, err)

	// neither the config the test still holds nor the one Config() handed out
	// may change what the mock reports
	cfg.MaxConns = 99
	cfg.ConnConfig.Database = "somewhere"
	mock.Config().MaxConns = 42
	mock.Config().ConnConfig.Database = "elsewhere"

	assert.Equal(t, int32(8), mock.Config().MaxConns)
	assert.Equal(t, "orders", mock.Config().ConnConfig.Database)
}
