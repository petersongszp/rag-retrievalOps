package common

import (
	"github.com/coze-dev/coze-studio/backend/domain/plugin/encrypt"
	"os"
)

const (
	// APIKeySecretEnv API密钥加密密钥环境变量
	// 注意：AES-128 需要 16 字节密钥，AES-256 需要 32 字节密钥
	APIKeySecretEnv = "USER_MODEL_API_KEY_SECRET"

	// DefaultAPIKeySecret 默认加密密钥（16字节，用于 AES-128）
	DefaultAPIKeySecret = "usermodel16bytes" // 正好 16 字节
)

// getSecret 获取加密密钥
func getSecret() string {
	secret := os.Getenv(APIKeySecretEnv)
	if secret == "" {
		secret = DefaultAPIKeySecret
	}
	return secret
}

// EncryptAPIKey 加密API密钥
// 使用项目现有的 AES-CBC 加密实现
func EncryptAPIKey(apiKey string) (string, error) {
	secret := getSecret()
	return encrypt.EncryptByAES([]byte(apiKey), secret)
}

// DecryptAPIKey 解密API密钥
// 使用项目现有的 AES-CBC 解密实现，支持向后兼容
func DecryptAPIKey(encrypted string) (string, error) {
	secret := getSecret()
	data, err := encrypt.DecryptByAES(encrypted, secret)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
