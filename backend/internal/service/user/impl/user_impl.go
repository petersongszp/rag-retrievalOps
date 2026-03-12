package impl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	userapi "interview-agents/api/model/user"
	"interview-agents/internal/config"
	"interview-agents/internal/m
	userapi "interview-agents/api/model/user"
	"interview-agents/internal/config"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

// ---------- GitHub OAuth ----------

func (s *UserServer) GitHubLogin(_ context.Context) (*userapi.WechatLoginQRResponse, error) {
	cid := config.Global.GitHub.ClientID
	redirect := config.Global.GitHub.RedirectURL
	// 若配置里仍是占位符（.env 未生效），则直接从环境变量读取
	if cid == "" || strings.Contains(cid, "${") {
		if v := os.Getenv("GITHUB_CLIENT_ID"); v != "" {
			cid = v
		}
	}
	if redirect == "" || strings.Contains(redirect, "${") {
		if v := os.Getenv("GITHUB_REDIRECT_URL"); v != "" {
			redirect = v
		}
	}
	if cid == "" || redirect == "" {
		return nil, errors.New("GitHub OAuth 未配置：请在项目根目录 .env 中设置 GITHUB_CLIENT_ID、GITHUB_REDIRECT_URL 并重启后端")
	}
	if strings.Contains(cid, "${") || strings.Contains(redirect, "${") {
		return nil, errors.New("GitHub OAuth 环境变量未生效：请检查 .env 格式（每行 KEY=值，等号两侧无空格），保存后重启后端")
	}
	state := fmt.Sprintf("github_%d", time.Now().UnixNano())
	loginURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
		url.QueryEscape(cid),
		url.QueryEscape(redirect),
		url.QueryEscape(state),
	)
	resp := userapi.NewWechatLoginQRResponse()
	resp.LoginURL = loginURL
	return resp, nil
}

func (s *UserServer) GitHubCallback(ctx context.Context, code string) (*userapi.LoginResponse, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("缺少授权码 code")
	}
	// 若 config 未展开，从环境变量兜底
	cid := config.Global.GitHub.ClientID
	secret := config.Global.GitHub.ClientSecret
	redirect := config.Global.GitHub.RedirectURL
	if cid == "" || strings.Contains(cid, "${") {
		cid = os.Getenv("GITHUB_CLIENT_ID")
	}
	if secret == "" || strings.Contains(secret, "${") {
		secret = os.Getenv("GITHUB_CLIENT_SECRET")
	}
	if redirect == "" || strings.Contains(redirect, "${") {
		redirect = os.Getenv("GITHUB_REDIRECT_URL")
	}
	if cid == "" || secret == "" || redirect == "" {
		return nil, errors.New("GitHub OAuth 未配置完整（请检查 .env 中的 GITHUB_CLIENT_ID、GITHUB_CLIENT_SECRET、GITHUB_REDIRECT_URL）")
	}

	token, err := s.getGitHubAccessTokenWithConfig(ctx, code, cid, secret, redirect)
	if err != nil {
		return nil, err
	}

	userInfo, err := s.getGitHubUserInfo(ctx, token)
	if err != nil {
		return nil, err
	}

	userRecord, err := s.githubLoginOrRegister(ctx, userInfo)
	if err != nil {
		return nil, err
	}

	jwtToken, err := middleware.GenerateToken(userRecord.ID, userRecord.Username, userRecord.Role)
	if err != nil {
		return nil, err
	}

	return s.buildLoginResponse(jwtToken, userRecord), nil
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

func (s *UserServer) getGitHubAccessTokenWithConfig(ctx context.Context, code, clientID, clientSecret, redirectURL string) (string, error) {
	body := url.Values{}
	body.Set("client_id", clientID)
	body.Set("client_secret", clientSecret)
	body.Set("code", code)
	body.Set("redirect_uri", redirectURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(body.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建 GitHub token 请求失败: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求 GitHub token 失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 GitHub token 响应失败: %w", err)
	}

	var tokenResp githubTokenResponse
	if err := json.Unmarshal(data, &tokenResp); err != nil {
		return "", fmt.Errorf("解析 GitHub token 响应失败: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("GitHub 授权失败: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("GitHub 未返回 access_token: %s", string(data))
	}

	return tokenResp.AccessToken, nil
}

func (s *UserServer) getGitHubAccessToken(ctx context.Context, code string) (string, error) {
	return s.getGitHubAccessTokenWithConfig(ctx, code,
		config.Global.GitHub.ClientID,
		config.Global.GitHub.ClientSecret,
		config.Global.GitHub.RedirectURL,
	)
}

type githubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func (s *UserServer) getGitHubUserInfo(ctx context.Context, accessToken string) (*githubUserInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("创建 GitHub 用户信息请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub 用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 GitHub 用户信息失败: %w", err)
	}

	var userInfo githubUserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, fmt.Errorf("解析 GitHub 用户信息失败: %w", err)
	}

	if userInfo.ID == 0 {
		return nil, fmt.Errorf("GitHub 用户信息不完整: %s", string(data))
	}

	return &userInfo, nil
}

func (s *UserServer) githubLoginOrRegister(_ context.Context, userInfo *githubUserInfo) (*model.User, error) {
	githubIDStr := fmt.Sprintf("%d", userInfo.ID)

	existingUser, err := model.UserDao.FindByGitHubID(githubIDStr)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询 GitHub 用户失败: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		username := firstNonEmpty(userInfo.Login, "github_"+githubIDStr)
		if username == "" {
			username = fmt.Sprintf("github_%d", time.Now().UnixNano())
		}
		email := firstNonEmpty(userInfo.Email, fmt.Sprintf("github_%s@placeholder.local", githubIDStr))
		nickname := firstNonEmpty(userInfo.Name, userInfo.Login)
		newUser := &model.User{
			Username:     username,
			Email:        email,
			PasswordHash: "",
			Role:         "user",
			GitHubID:     &githubIDStr,
			Nickname:     nickname,
			Avatar:       userInfo.AvatarURL,
		}
		if err := model.UserDao.Create(newUser); err != nil {
			return nil, fmt.Errorf("创建 GitHub 用户失败: %w", err)
		}
		return newUser, nil
	}

	updates := map[string]interface{}{
		"nickname": firstNonEmpty(userInfo.Name, userInfo.Login, existingUser.Nickname),
		"avatar":   firstNonEmpty(userInfo.AvatarURL, existingUser.Avatar),
	}
	if userInfo.Email != "" && existingUser.Email == "" {
		updates["email"] = userInfo.Email
	}
	if err := model.UserDao.UpdateByID(existingUser.ID, updates); err != nil {
		return nil, fmt.Errorf("更新 GitHub 用户信息失败: %w", err)
	}
	return model.UserDao.FindByID(existingUser.ID)
}
