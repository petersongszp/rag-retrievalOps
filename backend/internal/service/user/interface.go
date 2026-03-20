package user

import (
	"context"
	userapi "interview-agents/api/model/user"
	"interview-agents/internal/model"
	"interview-agents/internal/service/user/impl"
	"strings"
)

// NewModelManager 返回模型管理接口的默认实现
func NewModelManager() ModelManager {
	return impl.NewUserModelServer()
}

// NewUserManager 返回用户管理接口的默认实现
func NewUserManager() UserManager {
	return impl.NewUserServer()
}

type ModelManager interface {
	CreateUserModel(
		ctx context.Context,
		userID int64,
		req userapi.CreateUserModelRequest,
	) (string, error)
	ListUserModels(
		ctx context.Context,
		userID int64,
		req userapi.ListUserModelsRequest,
	) ([]*model.UserModel, int64, error)
	UserModelDetail(
		ctx context.Context,
		userID int64,
		modelID int64,
	) (*model.UserModel, error)
	UpdateUserModel(
		ctx context.Context,
		userID int64,
		req userapi.UpdateUserModelRequest,
	) error
	DeleteUserModel(
		ctx context.Context,
		userID int64,
		modelID int64,
	) error
	// CheckUserModelConfigured 检查用户是否配置了默认模型
	CheckUserModelConfigured(
		ctx context.Context,
		userID int64,
	) (*model.UserModel, error)
	// SwitchUserModel 手动切换用户模型
	SwitchUserModel(
		ctx context.Context,
		userID int64,
		oldModelID int64,
		newModelID int64,
	) error
}

type UserManager interface {
	Register(ctx context.Context, req userapi.RegisterRequest) (*userapi.LoginResponse, error)
	Login(ctx context.Context, req userapi.LoginRequest) (*userapi.LoginResponse, error)
	GetProfile(ctx context.Context, userID uint) (*userapi.UserProfile, error)
	UpdateProfile(ctx context.Context, userID uint, req userapi.UpdateProfileRequest) (*userapi.UserProfile, error)
	WechatLogin(ctx context.Context) (*userapi.WechatLoginQRResponse, error)
	WechatCallback(ctx context.Context, req userapi.WechatCallbackRequest) (*userapi.LoginResponse, error)
	GitHubLogin(ctx context.Context) (*userapi.WechatLoginQRResponse, error)
	GitHubCallback(ctx context.Context, code string) (*userapi.LoginResponse, error)
	GoogleLogin(ctx context.Context) (*userapi.WechatLoginQRResponse, error)
	GoogleCallback(ctx context.Context, code string) (*userapi.LoginResponse, error)
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}

func ToUserModelItem(m *model.UserModel) *userapi.UserModelItem {
	if m == nil {
		return nil
	}

	item := userapi.NewUserModelItem()
	item.ID = int64(m.ID)
	item.Name = m.Name
	item.ModelKey = m.ModelKey
	item.Protocol = m.Protocol
	item.BaseURL = m.BaseURL
	item.ProviderName = m.ProviderName

	if m.MetaID != 0 {
		metaID := m.MetaID
		item.MetaID = &metaID
	}
	if strings.TrimSpace(m.DefaultParams) != "" {
		defaultParams := m.DefaultParams
		item.DefaultParams = &defaultParams
	}
	if strings.TrimSpace(m.ConfigJSON) != "" {
		configJSON := m.ConfigJSON
		item.ConfigJSON = &configJSON
	}

	item.Scope = int32(m.Scope)
	item.Status = int32(m.Status)
	item.IsDefault = int32(m.IsDefault)
	item.CreatedAt = m.CreatedAt
	item.UpdatedAt = m.UpdatedAt
	item.HasSecret = strings.TrimSpace(m.APIKeyEncrypted) != ""
	if strings.TrimSpace(m.SecretHint) != "" {
		secretHint := m.SecretHint
		item.SecretHint = &secretHint
	}

	return item
}

func ToUserModelDetail(m *model.UserModel) *userapi.UserModelDetail {
	if m == nil {
		return nil
	}

	detail := userapi.NewUserModelDetail()
	detail.ID = int64(m.ID)
	detail.Name = m.Name
	detail.ModelKey = m.ModelKey
	detail.Protocol = m.Protocol
	detail.BaseURL = m.BaseURL
	detail.ProviderName = m.ProviderName

	if m.MetaID != 0 {
		metaID := m.MetaID
		detail.MetaID = &metaID
	}
	if strings.TrimSpace(m.DefaultParams) != "" {
		defaultParams := m.DefaultParams
		detail.DefaultParams = &defaultParams
	}
	if strings.TrimSpace(m.ConfigJSON) != "" {
		configJSON := m.ConfigJSON
		detail.ConfigJSON = &configJSON
	}

	detail.Scope = int32(m.Scope)
	detail.Status = int32(m.Status)
	detail.IsDefault = int32(m.IsDefault)
	detail.CreatedAt = m.CreatedAt
	detail.UpdatedAt = m.UpdatedAt
	detail.HasSecret = strings.TrimSpace(m.APIKeyEncrypted) != ""
	if strings.TrimSpace(m.SecretHint) != "" {
		secretHint := m.SecretHint
		detail.SecretHint = &secretHint
	}

	return detail
}
