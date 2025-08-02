package tmp

import (
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestService_Do(t *testing.T) {
	const (
		query       = "SELECT id FROM table"
		value int64 = 1
	)

	dao := NewMockDAO(t)
	svc := &Service{dao: dao}

	dao.EXPECT().
		QueryRow(mock.Anything, query).
		Return(
			pgxmock.NewRows([]string{"id"}).
				AddRow(value).
				Kind(),
		)

	// panic: runtime error: index out of range [-1]
	got, err := svc.Do(t.Context())

	require.NoError(t, err)
	assert.Equal(t, value, got)
}
