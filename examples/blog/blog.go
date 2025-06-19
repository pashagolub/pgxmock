package main

import (
	"context"
	"encoding/json"
	"net/http"

	pgx "github.com/jackc/pgx/v5"
	pgconn "github.com/jackc/pgx/v5/pgconn"
)

type PgxIface interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	Ping(context.Context) error
	Prepare(context.Context, string, string) (*pgconn.StatementDescription, error)
	Close(context.Context) error
}

type api struct {
	db PgxIface
}

type post struct {
	ID    int
	Title string
	Body  string
}

func (a *api) posts(w http.ResponseWriter, _ *http.Request) {
	rows, err := a.db.Query(context.Background(), "SELECT id, title, body FROM posts")
	if err != nil {
		a.fail(w, "failed to fetch posts: "+err.Error(), 500)
		return
	}
	defer rows.Close()

	var posts []*post
	for rows.Next() {
		p := &post{}
		if err := rows.Scan(&p.ID, &p.Title, &p.Body); err != nil {
			a.fail(w, "failed to scan post: "+err.Error(), 500)
			return
		}
		posts = append(posts, p)
	}
	if rows.Err() != nil {
		a.fail(w, "failed to read all posts: "+rows.Err().Error(), 500)
		return
	}

	data := struct {
		Posts []*post
	}{posts}

	a.ok(w, data)
}

func main() {
	// @NOTE: the real connection is not required for tests
	db, err := pgx.Connect(context.Background(), "postgres://postgres@localhost/blog")
	if err != nil {
		panic(err)
	}
	app := &api{db: db}
	http.HandleFunc("/posts", app.posts)
	_ = http.ListenAndServe(":8080", nil)
}

func (a *api) fail(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")

	data := struct {
		Error string
	}{Error: msg}

	resp, _ := json.Marshal(data)
	w.WriteHeader(status)
	_, _ = w.Write(resp)
}

func (a *api) ok(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		a.fail(w, "oops something evil has happened", 500)
		return
	}
	_, _ = w.Write(resp)
}
