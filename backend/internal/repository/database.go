package repository

import (
	"log"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the shared database handle for the RAG platform services.
var DB *gorm.DB

// InitDatabaseOnly initializes the database connection without auto-migrating.
// RAG services can decide explicitly when to run the narrowed RAG migration set.
func InitDatabaseOnly(dbConfig config.DatabaseConfig) error {
	logLevel := logger.Info

	db, err := gorm.Open(mysql.Open(dbConfig.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	if dbConfig.ConnMaxLifetime != "" {
		connMaxLifetime, err := time.ParseDuration(dbConfig.ConnMaxLifetime)
		if err == nil {
			sqlDB.SetConnMaxLifetime(connMaxLifetime)
		}
	}

	DB = db
	model.SetDBGetter(GetDB)

	log.Println("database initialized without auto-migration")
	return nil
}

// MigrateRAGDatabase migrates only the tables required by the RAG admin platform.
func MigrateRAGDatabase(db *gorm.DB) error {
	// RAG 账号、租户与 API Key 表
	if err := db.AutoMigrate(&model.RAGTenant{}, &model.RAGUser{}, &model.RAGAPIKey{}, &model.RAGTenantKBPermission{}); err != nil {
		log.Printf("[RAG-Server] Warning: RAG tenant/user/apikey auto migrate failed: %v", err)
	}

	return db.AutoMigrate(
		&model.KBKnowledgeBase{},
		&model.KBDocument{},
		&model.KBIngestJob{},
		&model.KBJobOperationLog{},
		&model.KBIndexRegistry{},
		&model.KBIndexOperationLog{},
		&model.KBRetrieveLog{},
		&model.KBCostTrace{},
		&model.KBAuditEvent{},
		&model.KBEvalDataset{},
		&model.KBEvalCase{},
		&model.KBEvalRun{},
	)
}

// GetDB returns the shared database instance.
func GetDB() *gorm.DB {
	return DB
}
