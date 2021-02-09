package pgxmock

// NewConn creates pgxmock database connection and a mock to manage expectations.
// Accepts options, like ValueConverterOption, to use a ValueConverter from
// a specific driver.
// Pings db so that all expectations could be
// asserted.
func NewConn(options ...func(*pgxmock) error) (Pgxmock, error) {
	smock := &pgxmock{ordered: true}
	return smock.open(options)
}

// NewWithDSN creates sqlmock database connection with a specific DSN
// and a mock to manage expectations.
// Accepts options, like ValueConverterOption, to use a ValueConverter from
// a specific driver.
// Pings db so that all expectations could be asserted.
//
// This method is introduced because of sql abstraction
// libraries, which do not provide a way to initialize
// with sql.DB instance. For example GORM library.
//
// Note, it will error if attempted to create with an
// already used dsn
//
// It is not recommended to use this method, unless you
// really need it and there is no other way around.
func NewWithDSN(dsn string, options ...func(*pgxmock) error) (Pgxmock, error) {
	smock := &pgxmock{dsn: dsn, ordered: true}
	return smock.open(options)
}
