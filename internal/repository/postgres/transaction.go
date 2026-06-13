package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/marquisccel/banking-peak-load-prototype/internal/domain/transaction"
)

type TransactionRepository struct {
	db      *sqlx.DB
	replica *sqlx.DB
}

func NewTransactionRepository(db, replica *sqlx.DB) *TransactionRepository {
	if replica == nil {
		replica = db
	}
	return &TransactionRepository{db: db, replica: replica}
}

func (r *TransactionRepository) Save(ctx context.Context, tx *transaction.Transaction) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO transactions
		    (id, source_account, dest_account, amount, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		tx.ID, tx.SourceAccount, tx.DestAccount, tx.Amount, string(tx.Status), tx.CreatedAt)
	return err
}

func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*transaction.Transaction, error) {
	var tx transaction.Transaction
	err := r.replica.GetContext(ctx, &tx,
		`SELECT id, source_account, dest_account, amount, status, created_at, updated_at
		 FROM transactions WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("transaction %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *TransactionRepository) UpdateStatus(ctx context.Context, id string, status transaction.Status) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transactions SET status = $1, updated_at = NOW() WHERE id = $2`,
		string(status), id)
	return err
}
