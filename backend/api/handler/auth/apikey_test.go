package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	authpkg "interview-agents/internal/auth"
	"interview-agents/internal/model"
	"interview-agents/internal/repository"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRotateAPIKeySuccess(t *testing.T) {
	db := setupAPIKeyHandlerTestDB(t)
	oldKey, oldHash, oldPrefix := seedAPIKeyFixture(t, db, 1, 11)
	_ = oldKey

	InitAPIKeyHandler(db)
	h := newAPIKeyHandlerTestServer(1, 7)

	resp := ut.PerformRequest(h.Engine, http.MethodPost, "/v1/api-keys/11/rotate", nil).Result()
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", resp.StatusCode(), string(resp.Body()))
	}

	var payload struct {
		Code int                          `json:"code"`
		Data authpkg.RotateAPIKeyResponse `json:"data"`
	}
	decodeAPIKeyJSONResponse(t, resp.Body(), &payload)
	if payload.Code != 200 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Data.ID != 11 {
		t.Fatalf("rotated id = %d, want 11", payload.Data.ID)
	}
	if !authpkg.ValidateAPIKeyFormat(payload.Data.Key) {
		t.Fatalf("returned key has invalid format: %q", payload.Data.Key)
	}

	repo := repository.NewRAGAPIKeyRepository(db)
	stored, err := repo.GetByIDForTenant(1, 11)
	if err != nil {
		t.Fatalf("failed to reload API key: %v", err)
	}

	if stored.KeyHash == oldHash {
		t.Fatal("expected key_hash to change after rotate")
	}
	if stored.KeyPrefix == oldPrefix {
		t.Fatal("expected key_prefix to change after rotate")
	}
	if stored.KeyHash != authpkg.HashAPIKey(payload.Data.Key) {
		t.Fatal("stored key_hash does not match returned plaintext key")
	}
	if stored.KeyPrefix != payload.Data.KeyPrefix {
		t.Fatalf("stored key_prefix = %q, want %q", stored.KeyPrefix, payload.Data.KeyPrefix)
	}
}

func TestRotateAPIKeyRejectsCrossTenantAccess(t *testing.T) {
	db := setupAPIKeyHandlerTestDB(t)
	_, oldHash, oldPrefix := seedAPIKeyFixture(t, db, 2, 22)

	InitAPIKeyHandler(db)
	h := newAPIKeyHandlerTestServer(1, 7)

	resp := ut.PerformRequest(h.Engine, http.MethodPost, "/v1/api-keys/22/rotate", nil).Result()
	if resp.StatusCode() != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body=%s", resp.StatusCode(), string(resp.Body()))
	}

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	decodeAPIKeyJSONResponse(t, resp.Body(), &payload)
	if payload.Code != 404 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	repo := repository.NewRAGAPIKeyRepository(db)
	stored, err := repo.GetByID(22)
	if err != nil {
		t.Fatalf("failed to reload API key: %v", err)
	}
	if stored.KeyHash != oldHash || stored.KeyPrefix != oldPrefix {
		t.Fatal("cross-tenant rotate attempt should not modify key material")
	}
}

func setupAPIKeyHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.RAGTenant{}, &model.RAGUser{}, &model.RAGAPIKey{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	tenants := []*model.RAGTenant{
		{ID: 1, Name: "Tenant One", Slug: "tenant-one", Plan: "pro", Status: "active"},
		{ID: 2, Name: "Tenant Two", Slug: "tenant-two", Plan: "pro", Status: "active"},
	}
	for _, tenant := range tenants {
		if err := db.Create(tenant).Error; err != nil {
			t.Fatalf("failed to seed tenant %d: %v", tenant.ID, err)
		}
	}

	users := []*model.RAGUser{
		{ID: 7, TenantID: 1, Email: "owner1@example.com", PasswordHash: "hash", Name: "Owner One", Role: "owner", Status: "active"},
		{ID: 8, TenantID: 2, Email: "owner2@example.com", PasswordHash: "hash", Name: "Owner Two", Role: "owner", Status: "active"},
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user %d: %v", user.ID, err)
		}
	}

	return db
}

func seedAPIKeyFixture(t *testing.T, db *gorm.DB, tenantID, keyID uint64) (string, string, string) {
	t.Helper()

	key, keyHash, keyPrefix, err := authpkg.GenerateAPIKey()
	if err != nil {
		t.Fatalf("failed to generate seed API key: %v", err)
	}

	userID := uint64(7)
	if tenantID == 2 {
		userID = 8
	}

	record := &model.RAGAPIKey{
		ID:          keyID,
		TenantID:    tenantID,
		UserID:      userID,
		AppID:       fmt.Sprintf("app-%d", tenantID),
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Name:        fmt.Sprintf("Key %d", keyID),
		Permissions: authpkg.FormatPermissions([]string{"retrieve"}),
		Status:      "active",
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to seed API key %d: %v", keyID, err)
	}

	return key, keyHash, keyPrefix
}

func newAPIKeyHandlerTestServer(tenantID uint64, userID uint) *server.Hertz {
	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		identity := &authpkg.Identity{
			AuthType: authpkg.AuthTypeJWT,
			TenantID: tenantID,
			UserID:   userID,
			Role:     "owner",
		}
		c.Next(authpkg.WithIdentity(ctx, identity))
	})
	h.POST("/v1/api-keys/:id/rotate", RotateAPIKey)
	return h
}

func decodeAPIKeyJSONResponse(t *testing.T, body []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, string(body))
	}
}
