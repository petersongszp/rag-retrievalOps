package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims 自定义 JWT Claims
type JWTClaims struct {
	UserID    uint64 `json:"user_id"`
	TenantID  uint64 `json:"tenant_id"`
	Role      string `json:"role"`
	AuthType  string `json:"auth_type"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Issuer     string
}

// JWTManager JWT 管理器
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(config JWTConfig) *JWTManager {
	if config.Issuer == "" {
		config.Issuer = "rag-platform"
	}
	return &JWTManager{config: config}
}

// GenerateAccessToken 生成 access token
func (m *JWTManager) GenerateAccessToken(userID, tenantID uint64, role string) (string, error) {
	return m.generateToken(userID, tenantID, role, "access", m.config.AccessTTL)
}

// GenerateRefreshToken 生成 refresh token
func (m *JWTManager) GenerateRefreshToken(userID, tenantID uint64, role string) (string, error) {
	return m.generateToken(userID, tenantID, role, "refresh", m.config.RefreshTTL)
}

func (m *JWTManager) generateToken(userID, tenantID uint64, role, tokenType string, ttl time.Duration) (string, error) {
	claims := JWTClaims{
		UserID:    userID,
		TenantID:  tenantID,
		Role:      role,
		AuthType:  "jwt",
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    m.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.Secret))
}

// ValidateToken 验证 token
func (m *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.config.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// ParseTokenClaims 解析 token（不验证过期）
func (m *JWTManager) ParseTokenClaims(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(m.config.Secret), nil
	}, jwt.WithoutClaimsValidation())

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	return claims, nil
}
