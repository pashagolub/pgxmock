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

// MonitorPingsOption determines whether calls to Ping on the driver should be
// observed and mocked.
//
// If true is passed, we will check these calls were expected. Expectations can
// be registered using the ExpectPing() method on the mock.
//
// If false is passed or this option is omitted, calls to Ping will not be
// considered when determining expectations and calls to ExpectPing will have
// no effect.
func MonitorPingsOption(monitorPings bool) func(*pgxmock) error {
	return func(s *pgxmock) error {
		s.monitorPings = monitorPings
		return nil
	}
}
