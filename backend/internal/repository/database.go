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

// DB 全局数据库实例
var DB *gorm.DB

// InitDatabase 初始化数据库连接
func InitDatabase(dbConfig config.DatabaseConfig) error {
	// 配置GORM日志
	logLevel := logger.Info

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dbConfig.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return err
	}

	// 配置连接池
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

	// 设置全局DB实例
	DB = db

	// 设置 model 包的 DB 获取函数
	model.SetDBGetter(GetDB)

	// 自动迁移数据库表结构
	err = migrateDatabase()
	if err != nil {
		return err
	}

	log.Println("数据库连接成功并完成迁移")
	return nil
}

// migrateDatabase 执行数据库迁移
func migrateDatabase() error {
	return DB.AutoMigrate(
		&model.User{},
		&model.UserModel{},
		&model.InterviewRecord{},
		&model.InterviewDialogue{},
		&model.InterviewEvaluation{},
		&model.AnswerReport{},
		&model.Resume{},
		&model.PredictionRecord{},
		&model.PredictionQuestion{},
		// 支付模块
		&model.PaymentOrder{},
		&model.PaymentAttempt{},
		&model.Subscription{},
		&model.PaymentEvent{},
		&model.PaymentCallback{},
		&model.KBKnowledgeBase{},
		&model.KBDocument{},
		&model.KBIngestJob{},
		&model.KBJobOperationLog{},
	)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}
