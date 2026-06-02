package auth

import "context"

// 角色常量
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// 权限常量
const (
	PermTenantRead  = "tenant:read"
	PermTenantWrite = "tenant:write"
	PermMemberRead  = "member:read"
	PermMemberWrite = "member:write"
	PermKBRead      = "kb:read"
	PermKBWrite     = "kb:write"
	PermLogRead     = "log:read"
	PermAPIKeyRead  = "api_key:read"
	PermAPIKeyWrite = "api_key:write"
)

// 角色权限映射
var rolePermissions = map[string][]string{
	RoleOwner: {
		PermTenantRead, PermTenantWrite,
		PermMemberRead, PermMemberWrite,
		PermKBRead, PermKBWrite,
		PermLogRead,
		PermAPIKeyRead, PermAPIKeyWrite,
	},
	RoleAdmin: {
		PermTenantRead,
		PermMemberRead, PermMemberWrite,
		PermKBRead, PermKBWrite,
		PermLogRead,
		PermAPIKeyRead, PermAPIKeyWrite,
	},
	RoleMember: {
		PermTenantRead,
		PermMemberRead,
		PermKBRead, PermKBWrite,
		PermLogRead,
		PermAPIKeyRead,
	},
	RoleViewer: {
		PermTenantRead,
		PermMemberRead,
		PermKBRead,
		PermLogRead,
	},
}

// HasPermission 检查角色是否有指定权限
func HasPermission(role, permission string) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// GetPermissions 获取角色的所有权限
func GetPermissions(role string) []string {
	return rolePermissions[role]
}

// RequirePermission 检查当前用户是否有指定权限
func RequirePermission(ctx context.Context, permission string) bool {
	identity := GetIdentity(ctx)
	return HasPermission(identity.Role, permission)
}
