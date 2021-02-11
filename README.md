[![GoDoc](https://pkg.go.dev/github.com/pashagolub/pgxmock?status.svg)](https://pkg.go.dev/github.com/pashagolub/pgxmock)
[![Go Report Card](https://goreportcard.com/badge/github.com/pashagolub/pgxmock)](https://goreportcard.com/report/github.com/pashagolub/pgxmock)
[![Coverage Status](https://coveralls.io/repos/github/pashagolub/pgxmock/badge.svg?branch=master)](https://coveralls.io/github/pashagolub/pgxmock?branch=master)


# pgx driver mock for Golang

**pgxmock** is a mock library implementing [pgx - PostgreSQL Driver and Toolkit](https://github.com/jackc/pgx/). 
It's based on the well-known [sqlmock](https://github.com/DATA-DOG/go-sqlmock) library for `sql/driver`.

**pgxmock** has one and only purpose - to simulate **pgx** behavior in tests, without needing a real database connection. It helps to maintain correct **TDD** workflow.

- this library is **not** complete and **not** stable (issues and pull requests are welcome);
- written based on **go1.15** bersion, however, should be compatible with **go1.11** and above;
- does not require any modifications to your source code;
- has strict by default expectation order matching;
- has no third party dependencies except **pgx** packages.

## Install

    go get github.com/pashagolub/pgxmock

## Documentation and Examples

Visit [godoc](http://pkg.go.dev/github.com/pashagolub/pgxmock) for general examples and public api reference.

See implementation examples:

- [blog API server](https://github.com/pashagolub/pgxmock/tree/master/examples/blog)
- [the same orders example](https://github.com/pashagolub/pgxmock/tree/master/examples/orders)

### Something you may want to test

``` go
package main

import (
	"context"

	pgx "github.com/jackc/pgx/v4"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Close(context.Context) error
}

func recordStats(db PgxIface, userID, productID int64) (err error) {
	tx, err := db.Begin(context.Background())
	if err != nil {
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

	if _, err = tx.Exec(context.Background(), "UPDATE products SET views = views + 1"); err != nil {
		return
	}
	if _, err = tx.Exec(context.Background(), "INSERT INTO product_viewers (user_id, product_id) VALUES (?, ?)", userID, productID); err != nil {
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

	"github.com/pashagolub/pgxmock"
)

// a successful case
func TestShouldUpdateStats(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("INSERT INTO product_viewers").WithArgs(2, 3).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	// now we execute our method
	if err = recordStats(mock, 2, 3); err != nil {
		t.Errorf("error was not expected while updating stats: %s", err)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// a failing test case
func TestShouldRollbackStatUpdatesOnFailure(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close(context.Background())

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE products").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
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
	mock, err := pgxmock.New(context.Background(), sqlmock.QueryMatcherOption(pgxmock.QueryMatcherEqual))
```

The query matcher can be fully customized based on user needs. **pgxmock** will not
provide a standard sql parsing matchers.

## Matching arguments like time.Time

There may be arguments which are of `struct` type and cannot be compared easily by value like `time.Time`. In this case
**pgxmock** provides an [Argument](https://pkg.go.dev/github.com/pashagolub/pgxmock#Argument) interface which
can be used in more sophisticated matching. Here is a simple example of time argument matching:

``` go
type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
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

## Change Log
- **2021-02-10** - public release of **pgxmock**.

Derived from **sqlmock**:
- **2019-04-06** - added functionality to mock a sql MetaData request
- **2019-02-13** - added `go.mod` removed the references and suggestions using `gopkg.in`.
- **2018-12-11** - added expectation of Rows to be closed, while mocking expected query.
- **2018-12-11** - introduced an option to provide **QueryMatcher** in order to customize SQL query matching.
- **2017-09-01** - it is now possible to expect that prepared statement will be closed,
  using **ExpectedPrepare.WillBeClosed**.
- **2017-02-09** - implemented support for **go1.8** features. **Rows** interface was changed to struct
  but contains all methods as before and should maintain backwards compatibility. **ExpectedQuery.WillReturnRows** may now
  accept multiple row sets.
- **2016-11-02** - `db.Prepare()` was not validating expected prepare SQL
  query. It should still be validated even if Exec or Query is not
  executed on that prepared statement.
- **2016-02-23** - added **sqlmock.AnyArg()** function to provide any kind
  of argument matcher.
- **2016-02-23** - convert expected arguments to driver.Value as natural
  driver does, the change may affect time.Time comparison and will be
  stricter. See [issue](https://github.com/DATA-DOG/go-sqlmock/issues/31).
- **2015-08-27** - **v1** api change, concurrency support, all known issues fixed.
- **2014-08-16** instead of **panic** during reflect type mismatch when comparing query arguments - now return error
- **2014-08-14** added **sqlmock.NewErrorResult** which gives an option to return driver.Result with errors for
interface methods, see [issue](https://github.com/DATA-DOG/go-sqlmock/issues/5)
- **2014-05-29** allow to match arguments in more sophisticated ways, by providing an **sqlmock.Argument** interface
- **2014-04-21** introduce **sqlmock.New()** to open a mock database connection for tests. This method
calls sql.DB.Ping to ensure that connection is open, see [issue](https://github.com/DATA-DOG/go-sqlmock/issues/4).
This way on Close it will surely assert if all expectations are met, even if database was not triggered at all.
The old way is still available, but it is advisable to call db.Ping manually before asserting with db.Close.
- **2014-02-14** RowsFromCSVString is now a part of Rows interface named as FromCSVString.
It has changed to allow more ways to construct rows and to easily extend this API in future.
See [issue 1](https://github.com/DATA-DOG/go-sqlmock/issues/1)
**RowsFromCSVString** is deprecated and will be removed in future

## Contributions

Feel free to open a pull request. Note, if you wish to contribute an extension to public (exported methods or types) -
please open an issue before, to discuss whether these changes can be accepted. All backward incompatible changes are
and will be treated cautiously

## License

The [three clause BSD license](http://en.wikipedia.org/wiki/BSD_licenses)

