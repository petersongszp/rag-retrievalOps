package auth

import (
	"context"
	"testing"
)

func TestIdentityContext(t *testing.T) {
	ctx := context.Background()

	// 测试默认值
	identity := GetIdentity(ctx)
	if identity.UserID != 0 {
		t.Errorf("expected user_id=0, got %d", identity.UserID)
	}

	// 测试注入和提取
	identity = &Identity{
		AuthType: AuthTypeJWT,
		TenantID: 1,
		UserID:   42,
		Role:     "admin",
		AppID:    "test-app",
	}
	ctx = WithIdentity(ctx, identity)

	if GetUserID(ctx) != 42 {
		t.Errorf("expected user_id=42, got %d", GetUserID(ctx))
	}
	if GetTenantID(ctx) != 1 {
		t.Errorf("expected tenant_id=1, got %d", GetTenantID(ctx))
	}
	if GetRole(ctx) != "admin" {
		t.Errorf("expected role=admin, got %s", GetRole(ctx))
	}
	if !IsAdmin(ctx) {
		t.Error("expected IsAdmin=true")
	}
	if IsLegacy(ctx) {
		t.Error("expected IsLegacy=false")
	}
}

func TestLegacyIdentity(t *testing.T) {
	ctx := context.Background()
	identity := &Identity{
		AuthType: AuthTypeLegacyAppID,
		AppID:    "interview-agent",
		IsLegacy: true,
	}
	ctx = WithIdentity(ctx, identity)

	if !IsLegacy(ctx) {
		t.Error("expected IsLegacy=true")
	}
	if GetAppID(ctx) != "interview-agent" {
		t.Errorf("expected app_id=interview-agent, got %s", GetAppID(ctx))
	}
}

func TestAuthTypeEnum(t *testing.T) {
	tests := []struct {
		authType AuthType
		expected string
	}{
		{AuthTypeJWT, "jwt"},
		{AuthTypeAPIKey, "api_key"},
		{AuthTypeLegacyAppID, "legacy_app_id"},
		{AuthTypeDevAdminBypass, "dev_admin_bypass"},
		{AuthTypeBootstrap, "bootstrap"},
	}

	for _, tt := range tests {
		if string(tt.authType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.authType))
		}
	}
}
