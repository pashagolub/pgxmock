package pgxmock

// ValueConverterOption allows to create a pgxmock connection
// with a custom ValueConverter to support drivers with special data types.
// func ValueConverterOption(converter driver.ValueConverter) func(*pgxmock) error {
// 	return func(s *pgxmock) error {
// 		s.converter = converter
// 		return nil
// 	}
// }

// QueryMatcherOption allows to customize SQL query matcher
// and match SQL query strings in more sophisticated ways.
// The default QueryMatcher is QueryMatcherRegexp.
func QueryMatcherOption(queryMatcher QueryMatcher) func(*pgxmock) error {
	return func(s *pgxmock) error {
		s.queryMatcher = queryMatcher
		return nil
	}
}
