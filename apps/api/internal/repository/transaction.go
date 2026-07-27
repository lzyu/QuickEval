package repository

import (
	"context"

	"gorm.io/gorm"
)

type TransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) TransactionManager {
	return TransactionManager{db: db}
}

func (manager TransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(tx *gorm.DB) error,
) error {
	return manager.db.WithContext(ctx).Transaction(fn)
}
