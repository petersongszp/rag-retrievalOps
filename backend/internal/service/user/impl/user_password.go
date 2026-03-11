package impl

import (
	"interview-agents/internal/model"
	"interview-agents/internal/repository"
	"interview-agents/internal/service/common"
	"context"
	"errors"
	"fmt"

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
			return errors.New("该邮箱未注册")
		}
		return err
	}

	// 2. 生成重置 Token
	token := uuid.New().String()

	// 3. 存入 Redis
	key := ResetTokenPrefix + token
	err = repository.SetCache(ctx, key, email, ResetTokenExpiration)
	if err != nil {
		return fmt.Errorf("生成凭证失败: %v", err)
	}

	// 4. 发送邮件
	// TODO: 从配置或环境变量获取前端地址
	resetLink := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", token)
	subject := "重置您的密码"
	body := fmt.Sprintf(`
		<h3>密码重置请求</h3>
		<p>您收到这封邮件是因为您请求重置密码。</p>
		<p>请点击下面的链接重置密码（15分钟内有效）：</p>
		<p><a href="%s">%s</a></p>
		<p>如果这不是您发起的请求，请忽略此邮件。</p>
	`, resetLink, resetLink)

	if err := common.SendEmail(email, subject, body); err != nil {
		return fmt.Errorf("发送邮件失败: %v", err)
	}

	return nil
}

// ResetPassword 处理重置密码
func (s *UserServer) ResetPassword(ctx context.Context, token, newPassword string) error {
	// 1. 验证 Token
	key := ResetTokenPrefix + token
	email, err := repository.GetCache(ctx, key)
	if err != nil {
		return errors.New("重置链接无效或已过期")
	}
	if email == "" {
		return errors.New("重置链接无效或已过期")
	}

	// 2. 查找用户
	user, err := model.UserDao.FindByEmail(email)
	if err != nil {
		return errors.New("用户不存在")
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
