[![Go Reference](https://pkg.go.dev/badge/github.com/pashagolub/pgxmock.svg)](https://pkg.go.dev/github.com/pashagolub/pgxmock/v4)
[![Go Report Card](https://goreportcard.com/badge/github.com/pashagolub/pgxmock)](https://goreportcard.com/report/github.com/pashagolub/pgxmock/v4)
[![Coverage Status](https://coveralls.io/repos/github/pashagolub/pgxmock/badge.svg?branch=master)](https://coveralls.io/github/pashagolub/pgxmock?branch=master)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

# pgx driver mock for Golang

**pgxmock** is a mock library implementing [pgx - PostgreSQL Driver and Toolkit](https://github.com/jackc/pgx/). 
It's based on the well-known [sqlmock](https://github.com/DATA-DOG/go-sqlmock) library for `sql/driver`.

**pgxmock** has one and only purpose - to simulate **pgx** behavior in tests, without needing a real database connection. It helps to maintain correct **TDD** workflow.

- written based on **go1.21** version;
- does not require any modifications to your source code;
- has strict by default expectation order matching;
- has no third party dependencies except **pgx** packages.

## Install

    go get github.com/pashagolub/pgxmock/v4

## Documentation and Examples

Visit [godoc](http://pkg.go.dev/github.com/pashagolub/pgxmock/v4) for general examples and public api reference.

See implementation examples:

- [the simplest one](https://github.com/pashagolub/pgxmock/tree/master/examples/basic)
- [blog API server](https://github.com/pashagolub/pgxmock/tree/master/examples/blog)


### Something you may want to test

``` go
package main

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Close(context.Context) error
}

func recordStats(db PgxIface, userID, productID int) (err error) {
	if tx, err := db.Begin(context.Background()); err != nil {
		return
	}
	defer func() {
		switch err {
		case nil:
			err = tx.Commit(context.Background())
		default:
			_ = tx.Rollback(context.Background())
		}
	}()
	sql := "UPDATE products SET views = views + 1"
	if _, err = tx.Exec(context.Background(), sql); err != nil {
		return
	}
	sql = "INSERT INTO product_viewers (user_id, product_id) VALUES ($1, $2)"
	if _, err = tx.Exec(context.Background(), sql, userID, productID); err != nil {
		return
	}
	return
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgx.Connect(context.Background(), "postgres://rolname@hostname/dbname")
	if err != nil {
		panic(err)
	}
	defer db.Close(context.Background())

	if err = recordStats(db, 1 /*some user id*/, 5 /*some product id*/); err != nil {
		panic(err)
	}
}
```

### Tests with pgxmock

``` go
package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

// a successful case
func TestShouldUpdateStats(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO product_viewers").
		WithArgs(2, 3).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	// now we execute our method
	if err = recordStats(mock, 2, 3); err != nil {
		t.Errorf("error was not expected while updating: %s", err)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// a failing test case
func TestShouldRollbackStatUpdatesOnFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO product_viewers").
		WithArgs(2, 3).
		WillReturnError(fmt.Errorf("some error"))
	mock.ExpectRollback()

	// now we execute our method
	if err = recordStats(mock, 2, 3); err == nil {
		t.Errorf("was expecting an error, but there was none")
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
```

## Customize SQL query matching

There were plenty of requests from users regarding SQL query string validation or different matching option.
We have now implemented the `QueryMatcher` interface, which can be passed through an option when calling
`pgxmock.New` or `pgxmock.NewWithDSN`.

This now allows to include some library, which would allow for example to parse and validate SQL AST.
And create a custom QueryMatcher in order to validate SQL in sophisticated ways.

By default, **pgxmock** is preserving backward compatibility and default query matcher is `pgxmock.QueryMatcherRegexp`
which uses expected SQL string as a regular expression to match incoming query string. There is an equality matcher:
`QueryMatcherEqual` which will do a full case sensitive match.

In order to customize the QueryMatcher, use the following:

``` go
	mock, err := pgxmock.New(context.Background(), pgxmock.QueryMatcherOption(pgxmock.QueryMatcherEqual))
```

The query matcher can be fully customized based on user needs. **pgxmock** will not
provide a standard sql parsing matchers.

## Matching arguments like time.Time

There may be arguments which are of `struct` type and cannot be compared easily by value like `time.Time`. In this case
**pgxmock** provides an [Argument](https://pkg.go.dev/github.com/pashagolub/pgxmock/v4#Argument) interface which
can be used in more sophisticated matching. Here is a simple example of time argument matching:

``` go
type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v interface{}) bool {
	_, ok := v.(time.Time)
	return ok
}

func TestAnyTimeArgument(t *testing.T) {
	t.Parallel()
	db, mock, err := New()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO users").
		WithArgs("john", AnyTime{}).
		WillReturnResult(NewResult(1, 1))

	_, err = db.Exec("INSERT INTO users(name, created_at) VALUES (?, ?)", "john", time.Now())
	if err != nil {
		t.Errorf("error '%s' was not expected, while inserting a row", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
```

It only asserts that argument is of `time.Time` type.

## Run tests

    go test -race

## Contributions

Feel free to open a pull request. Note, if you wish to contribute an extension to public (exported methods or types) -
please open an issue before, to discuss whether these changes can be accepted. All backward incompatible changes are
and will be treated cautiously

## License

The [three clause BSD license](http://en.wikipedia.org/wiki/BSD_licenses)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=pashagolub/pgxmock&type=Date)](https://star-history.com/#pashagolub/pgxmock&Date)

