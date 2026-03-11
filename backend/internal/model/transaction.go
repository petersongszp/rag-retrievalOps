package model

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// WithTransaction 在一个数据库事务中执行 fn，自动处理提交/回滚和 panic。
// - ctx 可为 nil；如果非 nil，则会通过 WithContext 绑定到事务。
// - fn 内部只需要返回 error，不需要手动 Commit / Rollback。
func WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) (err error) {
	if getDB == nil {
		panic("getDB function not initialized, please call model.SetDBGetter first")
	}

	db := getDB()
	if db == nil {
		return fmt.Errorf("database instance is nil")
	}

	if ctx != nil {
		db = db.WithContext(ctx)
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 兜底处理 panic：记录日志并确保事务回滚，然后继续抛出 panic
	defer func() {
		if r := recover(); r != nil {
			if rbErr := tx.Rollback().Error; rbErr != nil {
				log.Printf("[WithTransaction] rollback error after panic: %v", rbErr)
			}
			log.Printf("[WithTransaction] panic, transaction rolled back: %v", r)
		}
	}()

	// 执行业务逻辑
	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			log.Printf("[WithTransaction] rollback error after fn error: %v", rbErr)
		}
		return err
	}

	// 正常提交事务
	if err = tx.Commit().Error; err != nil {
		log.Printf("[WithTransaction] commit error: %v", err)
		return err
	}

	return nil
}
