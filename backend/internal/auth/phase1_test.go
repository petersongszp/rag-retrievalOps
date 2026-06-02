package auth

import (
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	password := "TestPassword@123"

	// 测试哈希
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// 测试验证
	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}

	// 测试错误密码
	if CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestPasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"short", true},                // 太短
		{"admin", true},                // 弱密码
		{"Admin@123", true},            // 弱密码
		{"ValidPassword@123", false},   // 合格
		{"AnotherValid@456", false},    // 合格
	}

	for _, tt := range tests {
		err := ValidatePasswordStrength(tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePasswordStrength(%q) error=%v, wantErr=%v", tt.password, err, tt.wantErr)
		}
	}
}

func TestJWTManager(t *testing.T) {
	mgr := NewJWTManager(JWTConfig{
		Secret:     "test-secret-key-for-testing",
		AccessTTL:  time.Hour,
		RefreshTTL: 24 * time.Hour,
		Issuer:     "test",
	})

	// 生成 access token
	accessToken, err := mgr.GenerateAccessToken(1, 1, "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// 验证 access token
	claims, err := mgr.ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("expected user_id=1, got %d", claims.UserID)
	}
	if claims.TenantID != 1 {
		t.Errorf("expected tenant_id=1, got %d", claims.TenantID)
	}
	if claims.Role != "owner" {
		t.Errorf("expected role=owner, got %s", claims.Role)
	}
	if claims.TokenType != "access" {
		t.Errorf("expected token_type=access, got %s", claims.TokenType)
	}

	// 生成 refresh token
	refreshToken, err := mgr.GenerateRefreshToken(1, 1, "owner")
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	// 验证 refresh token
	refreshClaims, err := mgr.ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if refreshClaims.TokenType != "refresh" {
		t.Errorf("expected token_type=refresh, got %s", refreshClaims.TokenType)
	}
}

func TestRBAC(t *testing.T) {
	tests := []struct {
		role       string
		permission string
		expected   bool
	}{
		{RoleOwner, PermTenantRead, true},
		{RoleOwner, PermTenantWrite, true},
		{RoleAdmin, PermTenantWrite, false},
		{RoleAdmin, PermKBWrite, true},
		{RoleMember, PermKBWrite, true},
		{RoleMember, PermMemberWrite, false},
		{RoleViewer, PermKBWrite, false},
		{RoleViewer, PermKBRead, true},
		{"unknown", PermKBRead, false},
	}

	for _, tt := range tests {
		result := HasPermission(tt.role, tt.permission)
		if result != tt.expected {
			t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.permission, result, tt.expected)
		}
	}
}
