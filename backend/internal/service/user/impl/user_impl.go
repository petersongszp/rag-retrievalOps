package impl

import (
	userapi "interview-agents/api/model/user"
	"interview-agents/internal/config"
	"interview-agents/internal/middleware"
	"interview-agents/internal/model"
	"interview-agents/internal/service/common"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

type UserServer struct {
	httpClient *http.Client
}

func NewUserServer() *UserServer {
	return &UserServer{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *UserServer) Register(_ context.Context, req userapi.RegisterRequest) (*userapi.LoginResponse, error) {
	_, err := model.UserDao.FindByUsernameOrEmail(req.GetUsername(), req.GetEmail())
	if err == nil {
		return nil, errors.New("用户名或邮箱已存在")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := common.HashPassword(req.GetPassword())
	if err != nil {
		return nil, err
	}

	userRecord := &model.User{
		Username:     req.GetUsername(),
		Email:        req.GetEmail(),
		PasswordHash: hash,
		Role:         "user",
	}

	if err := model.UserDao.Create(userRecord); err != nil {
		return nil, err
	}

	token, err := middleware.GenerateToken(userRecord.ID, userRecord.Username, userRecord.Role)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(token, userRecord), nil
}

func (s *UserServer) Login(_ context.Context, req userapi.LoginRequest) (*userapi.LoginResponse, error) {
	userRecord, err := model.UserDao.FindByEmail(req.GetEmail())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}

	// 验证密码（支持 bcrypt 和 明文兼容）
	if !common.CheckPasswordHash(req.GetPassword(), userRecord.PasswordHash) {
		// 如果 bcrypt 验证失败，尝试明文匹配（兼容旧数据）
		if userRecord.PasswordHash != req.GetPassword() {
			return nil, errors.New("密码错误")
		}
		// 如果明文匹配成功，自动升级为 bcrypt
		newHash, _ := common.HashPassword(req.GetPassword())
		model.UserDao.UpdateByID(userRecord.ID, map[string]interface{}{"password_hash": newHash})
	}

	token, err := middleware.GenerateToken(userRecord.ID, userRecord.Username, userRecord.Role)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(token, userRecord), nil
}

func (s *UserServer) GetProfile(_ context.Context, userID uint) (*userapi.UserProfile, error) {
	userRecord, err := model.UserDao.FindByID(userID)
	if err != nil {
		return nil, err
	}
	return s.toUserProfile(userRecord), nil
}

func (s *UserServer) UpdateProfile(ctx context.Context, userID uint, req userapi.UpdateProfileRequest) (*userapi.UserProfile, error) {
	updates := map[string]interface{}{}
	if req.IsSetUsername() {
		updates["username"] = req.GetUsername()
	}
	if req.IsSetEmail() {
		updates["email"] = req.GetEmail()
	}

	if err := model.UserDao.UpdateByID(userID, updates); err != nil {
		return nil, err
	}

	return s.GetProfile(ctx, userID)
}

func (s *UserServer) WechatLogin(_ context.Context) (*userapi.WechatLoginQRResponse, error) {
	loginURL := fmt.Sprintf(
		"https://open.weixin.qq.com/connect/qrconnect?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_login&state=STATE#wechat_redirect",
		config.Global.Wechat.AppID,
		url.QueryEscape(config.Global.Wechat.RedirectURL),
	)

	resp := userapi.NewWechatLoginQRResponse()
	resp.LoginURL = loginURL
	return resp, nil
}

func (s *UserServer) WechatCallback(ctx context.Context, req userapi.WechatCallbackRequest) (*userapi.LoginResponse, error) {
	if strings.TrimSpace(req.GetCode()) == "" {
		return nil, errors.New("缺少授权码")
	}

	tokenResp, err := s.getWechatAccessToken(ctx, req.GetCode())
	if err != nil {
		return nil, err
	}

	userInfo, err := s.getWechatUserInfo(ctx, tokenResp.AccessToken, tokenResp.OpenID)
	if err != nil {
		return nil, err
	}

	userRecord, err := s.wechatLoginOrRegister(ctx, tokenResp, userInfo)
	if err != nil {
		return nil, err
	}

	token, err := middleware.GenerateToken(userRecord.ID, userRecord.Username, userRecord.Role)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(token, userRecord), nil
}

func (s *UserServer) buildLoginResponse(token string, userRecord *model.User) *userapi.LoginResponse {
	resp := userapi.NewLoginResponse()
	resp.Token = token
	resp.User = s.toUserProfile(userRecord)
	return resp
}

func (s *UserServer) toUserProfile(userRecord *model.User) *userapi.UserProfile {
	if userRecord == nil {
		return nil
	}
	profile := userapi.NewUserProfile()
	profile.ID = int64(userRecord.ID)
	profile.Username = userRecord.Username
	profile.Email = userRecord.Email
	profile.Role = userRecord.Role

	if userRecord.WechatOpenID != nil {
		profile.WechatOpenID = userRecord.WechatOpenID
	}
	if userRecord.WechatUnionID != nil {
		profile.WechatUnionID = userRecord.WechatUnionID
	}
	if userRecord.Nickname != "" {
		profile.Nickname = &userRecord.Nickname
	}
	if userRecord.Avatar != "" {
		profile.Avatar = &userRecord.Avatar
	}

	if !userRecord.CreatedAt.IsZero() {
		val := userRecord.CreatedAt.UnixMilli()
		profile.CreatedAt = &val
	}
	if !userRecord.UpdatedAt.IsZero() {
		val := userRecord.UpdatedAt.UnixMilli()
		profile.UpdatedAt = &val
	}
	return profile
}

func (s *UserServer) getWechatAccessToken(ctx context.Context, code string) (*wechatTokenResponse, error) {
	reqURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		config.Global.Wechat.AppID,
		config.Global.Wechat.AppSecret,
		url.QueryEscape(code),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建微信授权请求失败: %w", err)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求微信授权接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取微信授权响应失败: %w", err)
	}

	var tokenResp wechatTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析微信授权响应失败: %w", err)
	}

	if tokenResp.ErrCode != 0 {
		return nil, fmt.Errorf("微信授权失败: %s", tokenResp.ErrMsg)
	}

	if tokenResp.AccessToken == "" || tokenResp.OpenID == "" {
		return nil, fmt.Errorf("微信授权响应不完整: %s", string(body))
	}

	return &tokenResp, nil
}

func (s *UserServer) getWechatUserInfo(ctx context.Context, accessToken, openID string) (*wechatUserInfo, error) {
	reqURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s",
		url.QueryEscape(accessToken),
		url.QueryEscape(openID),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建微信用户信息请求失败: %w", err)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求微信用户信息接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取微信用户信息响应失败: %w", err)
	}

	var userInfo wechatUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("解析微信用户信息响应失败: %w", err)
	}

	if userInfo.ErrCode != 0 {
		return nil, fmt.Errorf("获取微信用户信息失败: %s", userInfo.ErrMsg)
	}

	if userInfo.OpenID == "" {
		return nil, fmt.Errorf("微信用户信息不完整: %s", string(body))
	}

	return &userInfo, nil
}

func (s *UserServer) wechatLoginOrRegister(_ context.Context, tokenResp *wechatTokenResponse, userInfo *wechatUserInfo) (*model.User, error) {
	existingUser, err := model.UserDao.FindByWechatOpenID(userInfo.OpenID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询微信用户失败: %w", err)
	}

	unionID := firstNonEmpty(userInfo.UnionID, tokenResp.UnionID)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		username := generateWechatUsername(userInfo.OpenID)
		newUser := &model.User{
			Username:      username,
			Email:         "",
			PasswordHash:  "",
			Role:          "user",
			WechatOpenID:  &userInfo.OpenID,
			WechatUnionID: &unionID,
			Nickname:      userInfo.Nickname,
			Avatar:        userInfo.HeadImgURL,
		}

		if err := model.UserDao.Create(newUser); err != nil {
			return nil, fmt.Errorf("创建微信用户失败: %w", err)
		}
		return newUser, nil
	}

	updates := map[string]interface{}{
		"nickname": userInfo.Nickname,
		"avatar":   userInfo.HeadImgURL,
	}
	if existingUser.WechatUnionID == nil && unionID != "" {
		updates["wechat_union_id"] = unionID
	}

	if err := model.UserDao.UpdateByID(existingUser.ID, updates); err != nil {
		return nil, fmt.Errorf("更新微信用户信息失败: %w", err)
	}

	return model.UserDao.FindByID(existingUser.ID)
}

type wechatTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid,omitempty"`
	ErrCode      int    `json:"errcode,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
}

type wechatUserInfo struct {
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid,omitempty"`
	ErrCode    int      `json:"errcode,omitempty"`
	ErrMsg     string   `json:"errmsg,omitempty"`
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func generateWechatUsername(openID string) string {
	base := strings.TrimSpace(openID)
	if base == "" {
		return fmt.Sprintf("wechat_%d", time.Now().UnixNano())
	}
	if len(base) > 10 {
		base = base[:10]
	}
	return fmt.Sprintf("wechat_%s", base)
}
