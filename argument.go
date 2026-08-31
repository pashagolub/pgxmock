package pgxmock

import "reflect"

// Argument interface allows to match
// any argument in specific way when used with
// ExpectedQuery and ExpectedExec expectations.
type Argument interface {
	Match(any) bool
}

// ArgumentFunc type is an adapter to allow the use of ordinary functions as
// Argument. If f is a function with the appropriate signature,
// ArgumentFunc(f) is an Argument that calls f.
//
//	mock.ExpectExec("INSERT").WithArgs(pgxmock.ArgumentFunc(func(v any) bool {
//		id, ok := v.(int)
//		return ok && id > 0
//	}))
type ArgumentFunc func(any) bool

// Match implements the Argument interface
func (f ArgumentFunc) Match(v any) bool {
	return f(v)
}

// AnyArg will return an Argument which can
// match any kind of arguments.
//
// Useful for time.Time or similar kinds of arguments.
func AnyArg() Argument {
	return anyArgument{}
}

type anyArgument struct{}

func (a anyArgument) Match(_ any) bool {
	return true
}

// NotNil returns an Argument matching any value that is not nil, including a
// typed pointer, slice or map that happens to be nil-valued.
//
// Useful for an argument whose value is not predictable but whose presence
// matters, such as a generated identifier.
func NotNil() Argument {
	return ArgumentFunc(func(v any) bool {
		if v == nil {
			return false
		}
		switch val := reflect.ValueOf(v); val.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice,
			reflect.Func, reflect.Chan, reflect.UnsafePointer:
			return !val.IsNil()
		default:
			return true
		}
	})
}

// AnyOf returns an Argument matching any one of the given values, compared the
// way a plain expected argument is. Values may themselves be Arguments, so
// matchers can be combined:
//
//	WithArgs(pgxmock.AnyOf("pending", "active", pgxmock.NotNil()))
func AnyOf(values ...any) Argument {
	return ArgumentFunc(func(v any) bool {
		for _, want := range values {
			if matcher, ok := want.(Argument); ok {
				if matcher.Match(v) {
					return true
				}
				continue
			}
			if reflect.DeepEqual(want, v) {
				return true
			}
		}
		return false
	})
}

// OfType returns an Argument matching any value of type T, whatever its
// contents:
//
//	WithArgs(pgxmock.OfType[time.Time]())
//
// Unlike AnyArg this fails when the code under test passes the right value in
// the wrong type, which is a mistake a mock should catch.
func OfType[T any]() Argument {
	return ArgumentFunc(func(v any) bool {
		_, ok := v.(T)
		return ok
	})
}
