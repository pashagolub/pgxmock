package tmp

import (
	"context"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type (
	Service struct {
		dao DAO
	}

	DAO interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	}
)

func (svc *Service) Do(ctx context.Context) (int64, error) {
	query := squirrel.StatementBuilder.Select("id").From("table")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	row := svc.dao.QueryRow(ctx, sqlQuery, args...)

	var id int64

	err = row.Scan(&id)

	return id, err
}
