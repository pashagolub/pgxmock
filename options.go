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
