package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/marquisccel/banking-peak-load-prototype/internal/domain/account"
)

type AccountRepository struct {
	db      *sqlx.DB
	replica *sqlx.DB
}

func NewAccountRepository(db, replica *sqlx.DB) *AccountRepository {
	if replica == nil {
		replica = db
	}
	return &AccountRepository{db: db, replica: replica}
}

func (r *AccountRepository) GetByID(ctx context.Context, id int64) (*account.Account, error) {
	var a account.Account
	err := r.replica.GetContext(ctx, &a,
		`SELECT id, name, balance, updated_at FROM accounts WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("account %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
