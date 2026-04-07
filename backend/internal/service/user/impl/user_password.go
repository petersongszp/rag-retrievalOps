package impl

import (
	"context"
	"errors"
	"fmt"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"
	"interview-agents/internal/service/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ResetTokenPrefix     = "reset_pwd:"
	ResetTokenExpiration = 15 * 60 // 15 minutes in seconds
)

// ForgotPassword 处理忘记密码请求
func (s *UserServer) ForgotPassword(ctx context.Context, email string) error {
	// 1. 验证用户是否存在
	_, err := model.UserDao.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Email not registered")
		}
		return err
	}

	// 2. 生成重置 Token
	token := uuid.New().String()

	// 3. 存入 Redis
	key := ResetTokenPrefix + token
	err = repository.SetCache(ctx, key, email, ResetTokenExpiration)
	if err != nil {
		return fmt.Errorf("Failed to generate token: %v", err)
	}

	// 4. 发送邮件
	// TODO: 从配置或环境变量获取前端地址
	resetLink := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", token)
	subject := "Reset Your Password"
	body := fmt.Sprintf(`
		<h3>Password Reset Request</h3>
		<p>You received this email because you requested a password reset.</p>
		<p>Please click the link below to reset your password (valid for 15 minutes):</p>
		<p><a href="%s">%s</a></p>
		<p>If you did not request this, please ignore this email.</p>
	`, resetLink, resetLink)

	if err := common.SendEmail(email, subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %v", err)
	}

	return nil
}

// ResetPassword 处理重置密码
func (s *UserServer) ResetPassword(ctx context.Context, token, newPassword string) error {
	// 1. 验证 Token
	key := ResetTokenPrefix + token
	email, err := repository.GetCache(ctx, key)
	if err != nil {
		return errors.New("Reset link is invalid or has expired")
	}
	if email == "" {
		return errors.New("Reset link is invalid or has expired")
	}

	// 2. 查找用户
	user, err := model.UserDao.FindByEmail(email)
	if err != nil {
		return errors.New("User not found")
	}

	// 3. 加密新密码
	hash, err := common.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// 4. 更新密码
	updates := map[string]interface{}{
		"password_hash": hash,
	}
	if err := model.UserDao.UpdateByID(user.ID, updates); err != nil {
		return err
	}

	// 5. 删除 Token
	repository.DeleteCache(ctx, key)

	return nil
}