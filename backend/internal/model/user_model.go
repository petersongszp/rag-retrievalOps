package model

import (
	"fmt"
	"interview-agents/internal/errors"

	"gorm.io/gorm"
)

// getDB 获取数据库实例的函数变量，由 repository 包在初始化时设置
var getDB func() *gorm.DB

// SetDBGetter 设置数据库获取函数，由 repository 包在初始化时调用
func SetDBGetter(fn func() *gorm.DB) {
	getDB = fn
}

// checkDB 检查数据库是否已初始化
func checkDB() error {
	if getDB == nil {
		return errors.NewInternalError(
			"Database not initialized",
			fmt.Errorf("getDB function is nil, please call model.SetDBGetter first"),
		)
	}
	return nil
}

var UserModelDao _UserModel

type (
	_UserModel struct {
	}
	UserModel struct {
		ID              uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
		UserID          int64  `json:"user_id" gorm:"index;not null"`
		Name            string `json:"name" gorm:"size:128;not null;comment:模型显示名称（用户维度唯一）"`
		ModelKey        string `json:"model_key" gorm:"size:128;not null;comment:模型标识（doubao-1.5-vision-lite-250315）"`
		Protocol        string `json:"protocol" gorm:"size:64;not null;comment:协议类型（openai/ark/claude/gemini/deepseek/ollama/qwen/ernie）"`
		BaseURL         string `json:"base_url" gorm:"size:255;not null;comment:API 基础地址"`
		APIKeyEncrypted string `json:"api_key_encrypted" gorm:"type:text;not null;comment:加密后的 API 密钥"`
		ConfigJSON      string `json:"config_json" gorm:"type:json;comment:额外配置（如区域、访问密钥等）"`
		SecretHint      string `json:"secret_hint" gorm:"size:32;default:'';comment:密钥脱敏提示（如显示末尾4位）"`
		ProviderName    string `json:"provider_name" gorm:"size:64;not null;comment:提供商名称（如 OpenAI、Ark、DeepSeek）"`
		MetaID          int64  `json:"meta_id" gorm:"comment:关联全局 model_meta.id（继承能力/图标）"`
		DefaultParams   string `json:"default_params" gorm:"type:json;comment:默认参数（如 temperature、max_tokens）"`
		Scope           int    `json:"scope" gorm:"not null;default:7;comment:使用范围（位掩码：1=智能体, 2=应用, 4=工作流）"`
		Status          int    `json:"status" gorm:"not null;default:1;comment:状态（0=禁用, 1=启用）"`
		IsDefault       int    `json:"is_default" gorm:"not null;default:0;comment:是否为默认（0=不是, 1=是）"`
		CreatedAt       int64  `json:"created_at" gorm:"not null;default:0;comment:创建时间（毫秒时间戳）"`
		UpdatedAt       int64  `json:"updated_at" gorm:"not null;default:0;comment:更新时间（毫秒时间戳）"`
		Deleted         int    `json:"deleted" gorm:"not null;default:0;comment:删除状态（0=未删除, 1=已删除）"`
	}
)

// TableName 指定表名为 user_model（而不是默认的 usermodels）
func (u *UserModel) TableName() string {
	return "user_model"
}

// CreateUserModel 创建用户模型
func (u *_UserModel) CreateUserModel(data *UserModel) error {
	if err := checkDB(); err != nil {
		return err
	}
	err := getDB().Model(&UserModel{}).Create(data).Error
	if err != nil {
		return errors.NewDBError("Failed to create user model", err)
	}
	return nil
}

// ListUserModels 查询用户模型列表
func (u *_UserModel) ListUserModels(userID int64, page, pageSize int) ([]*UserModel, int64, error) {
	if err := checkDB(); err != nil {
		return nil, 0, err
	}
	var models []*UserModel
	var total int64

	query := getDB().Model(&UserModel{}).
		Where("user_id = ? AND deleted = ?", userID, 0)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDBError("Failed to count user models", err)
	}

	// 分页查询
	if err := query.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, 0, errors.NewDBError("Failed to list user models", err)
	}

	return models, total, nil
}

// GetUserModelByID 根据ID查询用户模型
func (u *_UserModel) GetUserModelByID(userID int64, modelID int64) (*UserModel, error) {
	if err := checkDB(); err != nil {
		return nil, err
	}
	var userModel UserModel
	err := getDB().Model(&UserModel{}).
		Where("id = ? AND user_id = ? AND deleted = ?", modelID, userID, 0).
		First(&userModel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("UserModel")
		}
		return nil, errors.NewDBError("Failed to get user model", err)
	}
	return &userModel, nil
}

// UpdateUserModel 更新用户模型
func (u *_UserModel) UpdateUserModel(data *UserModel) error {
	if err := checkDB(); err != nil {
		return err
	}
	err := getDB().Model(&UserModel{}).
		Where("id = ? AND user_id = ? AND deleted = ?", data.ID, data.UserID, 0).
		Updates(data).Error
	if err != nil {
		return errors.NewDBError("Failed to update user model", err)
	}
	return nil
}

// DeleteUserModel 软删除用户模型
func (u *_UserModel) DeleteUserModel(userID int64, modelID int64) error {
	if err := checkDB(); err != nil {
		return err
	}
	err := getDB().Model(&UserModel{}).
		Where("id = ? AND user_id = ? AND deleted = ?", modelID, userID, 0).
		Update("deleted", 1).Error
	if err != nil {
		return errors.NewDBError("Failed to delete user model", err)
	}
	return nil
}

// SetDefaultUserModel 设置用户的默认模型（同时取消其他模型的默认状态）
func (u *_UserModel) SetDefaultUserModel(userID int64, modelID int64) error {
	if err := checkDB(); err != nil {
		return err
	}

	tx := getDB().Begin()

	// 1. 先将该用户的所有模型的 is_default 设置为 0
	if err := tx.Model(&UserModel{}).
		Where("user_id = ? AND deleted = ?", userID, 0).
		Update("is_default", 0).Error; err != nil {
		tx.Rollback()
		return errors.NewDBError("Failed to reset default user models", err)
	}

	// 2. 再将指定模型的 is_default 设置为 1
	if err := tx.Model(&UserModel{}).
		Where("id = ? AND user_id = ? AND deleted = ?", modelID, userID, 0).
		Update("is_default", 1).Error; err != nil {
		tx.Rollback()
		return errors.NewDBError("Failed to set default user model", err)
	}

	if err := tx.Commit().Error; err != nil {
		return errors.NewDBError("Failed to commit transaction", err)
	}
	return nil
}

// GetDefaultUserModel 获取用户的默认模型
func (u *_UserModel) GetDefaultUserModel(userID int64) (*UserModel, error) {
	if err := checkDB(); err != nil {
		return nil, err
	}
	var userModel UserModel
	err := getDB().Model(&UserModel{}).
		Where("user_id = ? AND is_default = ? AND deleted = ?", userID, 1, 0).
		First(&userModel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("DefaultUserModel")
		}
		return nil, errors.NewDBError("Failed to get default user model", err)
	}

	return &userModel, nil
}

// CancelDefaultUserModel 取消用户模型的默认状态
// 如果 modelID = 0，则取消该用户所有模型的默认状态
// 如果 modelID > 0，则只取消指定模型的默认状态
func (u *_UserModel) CancelDefaultUserModel(userID int64, modelID int64) error {
	if err := checkDB(); err != nil {
		return err
	}

	query := getDB().Model(&UserModel{}).
		Where("user_id = ? AND deleted = ?", userID, 0)

	// 如果 modelID > 0，则只更新指定模型
	if modelID > 0 {
		query = query.Where("id = ?", modelID)
	}

	err := query.Update("is_default", 0).Error
	if err != nil {
		return errors.NewDBError("Failed to cancel default user model", err)
	}
	return nil
}

func (u *_UserModel) SetEnabledUserModel(userID int64, modelID int64) error {
	if err := checkDB(); err != nil {
		return err
	}

	tx := getDB().Begin()

	if err := tx.Model(&UserModel{}).
		Where("user_id = ? AND deleted = ?", userID, 0).
		Update("status", 0).Error; err != nil {
		tx.Rollback()
		return errors.NewDBError("Failed to disable user models", err)
	}

	if err := tx.Model(&UserModel{}).
		Where("id = ? AND user_id = ? AND deleted = ?", modelID, userID, 0).
		Update("status", 1).Error; err != nil {
		tx.Rollback()
		return errors.NewDBError("Failed to enable user model", err)
	}

	if err := tx.Commit().Error; err != nil {
		return errors.NewDBError("Failed to commit transaction", err)
	}
	return nil
}

// SwitchUserModel 手动切换模型
// 将 oldModelID 设置为 status=0, is_default=0
// 将 newModelID 设置为 status=1, is_default=1
// 原子操作保证状态一致性。如果未提供 oldModelID (<= 0)，则全量将当前用户的默认模型卸任。
func (u *_UserModel) SwitchUserModel(userID int64, oldModelID, newModelID int64) error {
	if err := checkDB(); err != nil {
		return err
	}

	tx := getDB().Begin()

	// 1. 废弃原主模型
	if oldModelID > 0 {
		if err := tx.Model(&UserModel{}).
			Where("id = ? AND user_id = ?", oldModelID, userID).
			Updates(map[string]interface{}{"status": 0, "is_default": 0}).Error; err != nil {
			tx.Rollback()
			return errors.NewDBError("Failed to disable old model", err)
		}
	} else {
		// 如果未明确老模型，兜底将目前生效的所有主模型卸下
		if err := tx.Model(&UserModel{}).
			Where("user_id = ? AND is_default = 1", userID).
			Updates(map[string]interface{}{"status": 0, "is_default": 0}).Error; err != nil {
			tx.Rollback()
			return errors.NewDBError("Failed to clear current default models", err)
		}
	}

	// 2. 启用新模型作为主模型
	if err := tx.Model(&UserModel{}).
		Where("id = ? AND user_id = ?", newModelID, userID).
		Updates(map[string]interface{}{"status": 1, "is_default": 1}).Error; err != nil {
		tx.Rollback()
		return errors.NewDBError("Failed to set new model as default", err)
	}

	if err := tx.Commit().Error; err != nil {
		return errors.NewDBError("Failed to commit SwitchUserModel transaction", err)
	}

	return nil
}

// ListBackupModels 查询用户可作为备选的模型列表
// 条件: deleted = 0。由于我们要提供候选，哪怕它之前 status=0，只要用户配置了，就可以作为候选项呈现。
// 通常可以排除掉指定的、当前坏掉的 failedModelID
func (u *_UserModel) ListBackupModels(userID int64, failedModelID int64) ([]*UserModel, error) {
	if err := checkDB(); err != nil {
		return nil, err
	}
	var models []*UserModel

	query := getDB().Model(&UserModel{}).
		Where("user_id = ? AND deleted = ?", userID, 0)

	if failedModelID > 0 {
		query = query.Where("id != ?", failedModelID)
	}

	err := query.Order("created_at DESC").Find(&models).Error
	if err != nil {
		return nil, errors.NewDBError("Failed to list backup models", err)
	}

	return models, nil
}
