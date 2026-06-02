package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLength = 12
	BcryptCost        = 12
)

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) (string, error) {
	if err := ValidatePasswordStrength(password); err != nil {
		return "", err
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePasswordStrength 校验密码强度
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}

	weakPasswords := []string{
		"admin", "admin123", "password", "123456", "qwerty",
		"Admin@123", "Password1", "1234567890",
	}
	for _, weak := range weakPasswords {
		if password == weak {
			return fmt.Errorf("password is too common")
		}
	}

	return nil
}
