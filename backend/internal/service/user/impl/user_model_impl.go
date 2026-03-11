package impl

import (
	"context"
	"errors"
	"interview-agents/api/model/user"
	"interview-agents/internal/model"
	"interview-agents/internal/service/common"
	"time"

	"gorm.io/gorm"
)

type UserModelServer struct {
}

func NewUserModelServer() *UserModelServer {
	return &UserModelServer{}
}

// CreateUserModel 创建用户模型
func (s *UserModelServer) CreateUserModel(ctx context.Context,
	userID int64,
	req user.CreateUserModelRequest) (string, error) {
	apiKey, err := common.EncryptAPIKey(req.APIKey)
	if err != nil {
		return "fail", errors.New("密钥加密失败")
	}

	// 处理可选字段
	var metaID uint64
	if req.IsSetMetaID() {
		metaID = uint64(*req.MetaID)
	}

	var configJSON string
	if req.IsSetConfigJSON() {
		configJSON = *req.ConfigJSON
	}

	var defaultParams string
	if req.IsSetDefaultParams() {
		defaultParams = *req.DefaultParams
	}

	scope := 7 // 默认值
	if req.IsSetScope() {
		scope = int(*req.Scope)
	}

	status := 1 // 默认值
	if req.IsSetStatus() {
		status = int(*req.Status)
	}

	// 检查是否需要设置为默认模型
	isDefault := 0
	if req.IsSetIsDefault() {
		isDefault = int(req.GetIsDefault())
	}

	// 如果设置为默认，先取消其他模型的默认状态
	if isDefault == 1 {
		_ = model.UserModelDao.CancelDefaultUserModel(userID, 0) // 0 表示取消所有
	}

	newModel := &model.UserModel{
		UserID:          userID,
		Name:            req.GetName(),
		ModelKey:        req.GetModelKey(),
		Protocol:        req.GetProtocol(),
		BaseURL:         req.GetBaseURL(),
		APIKeyEncrypted: apiKey,
		ConfigJSON:      configJSON,
		MetaID:          int64(metaID),
		DefaultParams:   defaultParams,
		Scope:           scope,
		Status:          status,
		IsDefault:       isDefault,
		ProviderName:    req.GetProviderName(),
	}
	err = model.UserModelDao.CreateUserModel(newModel)
	if err != nil {
		return "fail", err
	}
	if status == 1 {
		_ = model.UserModelDao.SetEnabledUserModel(userID, int64(newModel.ID))
	}
	return "success", nil
}

// ListUserModels 列出用户模型
func (s *UserModelServer) ListUserModels(ctx context.Context,
	userID int64,
	req user.ListUserModelsRequest) ([]*model.UserModel, int64, error) {
	page := int(req.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.GetSize())
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	return model.UserModelDao.ListUserModels(userID, page, pageSize)
}

// UserModelDetail 获取用户模型详情
func (s *UserModelServer) UserModelDetail(ctx context.Context,
	userID int64,
	modelID int64) (*model.UserModel, error) {
	return model.UserModelDao.GetUserModelByID(userID, modelID)
}

// UpdateUserModel 更新用户模型
func (s *UserModelServer) UpdateUserModel(ctx context.Context,
	userID int64,
	req user.UpdateUserModelRequest) error {
	// 先获取现有模型
	existingModel, err := model.UserModelDao.GetUserModelByID(userID, req.GetID())
	if err != nil {
		return err
	}

	// 更新字段
	existingModel.Name = req.GetName()
	existingModel.ModelKey = req.GetModelKey()
	existingModel.Protocol = req.GetProtocol()
	existingModel.BaseURL = req.GetBaseURL()
	existingModel.ProviderName = req.GetProviderName()

	// 处理可选字段 - APIKey
	if req.IsSetAPIKey() {
		apiKey, err := common.EncryptAPIKey(req.GetAPIKey())
		if err != nil {
			return errors.New("密钥加密失败")
		}
		existingModel.APIKeyEncrypted = apiKey
	}

	// 处理可选字段 - MetaID
	if req.IsSetMetaID() {
		existingModel.MetaID = req.GetMetaID()
	}

	// 处理可选字段 - DefaultParams
	if req.IsSetDefaultParams() {
		existingModel.DefaultParams = req.GetDefaultParams()
	}

	// 处理可选字段 - ConfigJSON
	if req.IsSetConfigJSON() {
		existingModel.ConfigJSON = req.GetConfigJSON()
	}

	// 处理可选字段 - Scope
	if req.IsSetScope() {
		existingModel.Scope = int(req.GetScope())
	}

	// 处理可选字段 - Status
	if req.IsSetStatus() {
		existingModel.Status = int(req.GetStatus())
	}

	// 处理可选字段 - IsDefault（支持 1 设为默认、0 取消默认）
	var isDefaultProvided bool
	var isDefaultValue int
	if req.IsSetIsDefault() {
		isDefaultProvided = true
		isDefaultValue = int(req.GetIsDefault())
		existingModel.IsDefault = isDefaultValue
	}

	existingModel.UpdatedAt = time.Now().UnixMilli()
	if err := model.UserModelDao.UpdateUserModel(existingModel); err != nil {
		return err
	}

	if isDefaultProvided {
		if isDefaultValue == 1 {
			if err := model.UserModelDao.SetDefaultUserModel(userID, int64(existingModel.ID)); err != nil {
				return err
			}
		} else {
			if err := model.UserModelDao.CancelDefaultUserModel(userID, int64(existingModel.ID)); err != nil {
				return err
			}
		}
	}
	if req.IsSetStatus() && int(req.GetStatus()) == 1 {
		if err := model.UserModelDao.SetEnabledUserModel(userID, int64(existingModel.ID)); err != nil {
			return err
		}
	}
	return nil
}

// DeleteUserModel 删除用户模型
func (s *UserModelServer) DeleteUserModel(ctx context.Context,
	userID int64,
	modelID int64) error {
	return model.UserModelDao.DeleteUserModel(userID, modelID)
}

// CheckUserModelConfigured 检查用户是否配置了默认模型
// 返回默认模型信息，如果为 nil 则表示未配置默认模型
func (s *UserModelServer) CheckUserModelConfigured(ctx context.Context,
	userID int64) (*model.UserModel, error) {
	defaultModel, err := model.UserModelDao.GetDefaultUserModel(userID)
	if err != nil {
		// 如果没有找到默认模型（IsDefault = 1），返回 nil
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		// 其他错误返回错误信息
		return nil, err
	}
	// GetDefaultUserModel 已经确保返回的是 is_default = 1 的模型
	// 直接返回模型信息，前端可以通过判断 model 是否为 null 来判断是否配置
	return defaultModel, nil
}

// SwitchUserModel 手动切换用户模型
func (s *UserModelServer) SwitchUserModel(ctx context.Context,
	userID int64,
	oldModelID int64,
	newModelID int64) error {

	// 如果提供了 newModelID，先检查该模型是否属于该用户且未被删除
	if newModelID > 0 {
		_, err := model.UserModelDao.GetUserModelByID(userID, newModelID)
		if err != nil {
			return err
		}
	}

	return model.UserModelDao.SwitchUserModel(userID, oldModelID, newModelID)
}
