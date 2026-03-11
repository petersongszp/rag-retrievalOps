package middleware

import (
	"context"
	"errors"
	"strings"
	"time"

	"interview-agents/api/response"
	"interview-agents/internal/config"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims 自定义JWT声明结构
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTSkipper func(ctx *app.RequestContext) bool

var (
	errAuthorizationHeaderRequired = errors.New("authorization header is required")
	errAuthorizationFormatInvalid  = errors.New("authorization header format must be Bearer {token}")
	errTokenNotFound               = errors.New("token not found")
)

// JWTMiddleware JWT认证中间件（兼容默认行为）
func JWTMiddleware() app.HandlerFunc {
	return JWTMiddlewareWithSkipper(nil)
}

// JWTMiddlewareWithSkipper JWT认证中间件，支持跳过特定请求
func JWTMiddlewareWithSkipper(skipper JWTSkipper) app.HandlerFunc {
	if skipper == nil {
		skipper = func(*app.RequestContext) bool { return false }
	}

	return func(c context.Context, ctx *app.RequestContext) {
		if skipper(ctx) {
			ctx.Next(c)
			return
		}

		tokenString, err := extractToken(ctx)
		if err != nil {
			message := "Authorization header is required"
			switch {
			case errors.Is(err, errAuthorizationFormatInvalid):
				message = "Authorization header format must be Bearer {token}"
			case errors.Is(err, errTokenNotFound):
				message = "Authorization token is required"
			}

			response.Unauthorized(c, ctx, message)
			ctx.Abort()
			return
		}

		claims, err := parseToken(tokenString)
		if err != nil {
			response.Unauthorized(c, ctx, "Invalid or expired token")
			ctx.Abort()
			return
		}

		ctx.Set("jwt_claims", claims)
		ctx.Set("user_id", claims.UserID)
		ctx.Set("username", claims.Username)
		ctx.Set("role", claims.Role)

		ctx.Next(c)
	}
}

// GenerateToken 生成JWT token
func GenerateToken(userID uint, username, role string) (string, error) {
	cfg := config.Global.Security

	// 解析过期时间
	expiration, err := time.ParseDuration(cfg.JWTExpiration)
	if err != nil {
		expiration = 24 * time.Hour // 默认24小时
	}

	// 创建声明
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "interview-agent",
		},
	}

	// 创建token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名并获取完整的编码后的字符串token
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// parseToken 解析JWT token
func parseToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.Global.Security.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetUserID 从上下文获取用户ID
func GetUserID(ctx *app.RequestContext) uint {
	userID, exists := ctx.Get("user_id")
	if !exists {
		return 0
	}
	return userID.(uint)
}

// GetUsername 从上下文获取用户名
func GetUsername(ctx *app.RequestContext) string {
	username, exists := ctx.Get("username")
	if !exists {
		return ""
	}
	return username.(string)
}

// GetUserRole 从上下文获取用户角色
func GetUserRole(ctx *app.RequestContext) string {
	role, exists := ctx.Get("role")
	if !exists {
		return ""
	}
	return role.(string)
}

// ParseAndSetUserFromToken 手动解析token并设置用户信息到上下文
// 用于跳过JWT中间件的接口中手动验证token
// 返回 userID，如果解析失败返回 0
func ParseAndSetUserFromToken(ctx *app.RequestContext) uint {
	tokenString, err := extractToken(ctx)
	if err != nil {
		return 0
	}

	claims, err := parseToken(tokenString)
	if err != nil {
		return 0
	}

	// 设置到上下文中
	ctx.Set("jwt_claims", claims)
	ctx.Set("user_id", claims.UserID)
	ctx.Set("username", claims.Username)
	ctx.Set("role", claims.Role)

	return claims.UserID
}

func extractToken(ctx *app.RequestContext) (string, error) {
	authHeader := strings.TrimSpace(string(ctx.GetHeader("Authorization")))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token := strings.TrimSpace(parts[1])
			if token != "" {
				return token, nil
			}
		}
		return "", errAuthorizationFormatInvalid
	}

	if tokenHeader := strings.TrimSpace(string(ctx.GetHeader("X-Auth-Token"))); tokenHeader != "" {
		return tokenHeader, nil
	}

	if queryToken := strings.TrimSpace(string(ctx.Query("token"))); queryToken != "" {
		return queryToken, nil
	}

	if cookieToken := ctx.Cookie("token"); len(cookieToken) > 0 {
		if token := strings.TrimSpace(string(cookieToken)); token != "" {
			return token, nil
		}
	}

	return "", errTokenNotFound
}
