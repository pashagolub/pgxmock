package pgxmock

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
