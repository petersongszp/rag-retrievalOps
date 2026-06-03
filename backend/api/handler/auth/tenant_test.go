package auth

import (
	"context"
	"encoding/json"
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

func TestTenantEndpointsReturnCurrentTenantContract(t *testing.T) {
	db := setupTenantHandlerTestDB(t)
	seedTenantUsageFixtures(t, db)

	tenantRepo = repository.NewRAGTenantRepository(db)
	userRepo = repository.NewRAGUserRepository(db)
	InitTenantHandler(db)

	h := newTenantHandlerTestServer()

	tenantResp := ut.PerformRequest(h.Engine, http.MethodGet, "/v1/tenant?tenant_id=999", nil).Result()
	if tenantResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected tenant status 200, got %d body=%s", tenantResp.StatusCode(), string(tenantResp.Body()))
	}

	var tenantPayload struct {
		Code int                    `json:"code"`
		Data authpkg.TenantResponse `json:"data"`
	}
	decodeTenantJSONResponse(t, tenantResp.Body(), &tenantPayload)
	if tenantPayload.Code != 200 {
		t.Fatalf("unexpected tenant payload: %+v", tenantPayload)
	}
	if tenantPayload.Data.TenantID != 1 {
		t.Fatalf("tenant_id = %d, want 1", tenantPayload.Data.TenantID)
	}
	if tenantPayload.Data.Name != "Tenant One" || tenantPayload.Data.Slug != "tenant-one" {
		t.Fatalf("unexpected tenant identity fields: %+v", tenantPayload.Data)
	}
	if tenantPayload.Data.MaxKBCount != 8 || tenantPayload.Data.MaxDocCount != 80 || tenantPayload.Data.MaxStorageMB != 64 || tenantPayload.Data.MaxAPICallsPerDay != 800 {
		t.Fatalf("unexpected tenant limits: %+v", tenantPayload.Data)
	}

	usageResp := ut.PerformRequest(h.Engine, http.MethodGet, "/v1/tenant/usage?tenant_id=2", nil).Result()
	if usageResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected usage status 200, got %d body=%s", usageResp.StatusCode(), string(usageResp.Body()))
	}

	var usagePayload struct {
		Code int                         `json:"code"`
		Data authpkg.TenantUsageResponse `json:"data"`
	}
	decodeTenantJSONResponse(t, usageResp.Body(), &usagePayload)
	if usagePayload.Code != 200 {
		t.Fatalf("unexpected usage payload: %+v", usagePayload)
	}
	if usagePayload.Data.KBCount != 2 || usagePayload.Data.DocCount != 2 {
		t.Fatalf("unexpected usage counts: %+v", usagePayload.Data)
	}
	if usagePayload.Data.StorageMB != 4 {
		t.Fatalf("storage_mb = %d, want 4", usagePayload.Data.StorageMB)
	}
	if usagePayload.Data.Limits.MaxKBCount != 8 || usagePayload.Data.Limits.MaxDocCount != 80 || usagePayload.Data.Limits.MaxStorageMB != 64 || usagePayload.Data.Limits.MaxAPICallsPerDay != 800 {
		t.Fatalf("unexpected usage limits: %+v", usagePayload.Data.Limits)
	}
}

func setupTenantHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.RAGTenant{}, &model.RAGUser{}, &model.KBKnowledgeBase{}, &model.KBDocument{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return db
}

func seedTenantUsageFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()

	tenantOne := &model.RAGTenant{
		ID:                1,
		Name:              "Tenant One",
		Slug:              "tenant-one",
		Plan:              "pro",
		Status:            "active",
		MaxKBCount:        8,
		MaxDocCount:       80,
		MaxStorageMB:      64,
		MaxAPICallsPerDay: 800,
	}
	tenantTwo := &model.RAGTenant{
		ID:                2,
		Name:              "Tenant Two",
		Slug:              "tenant-two",
		Plan:              "free",
		Status:            "active",
		MaxKBCount:        3,
		MaxDocCount:       30,
		MaxStorageMB:      16,
		MaxAPICallsPerDay: 300,
	}
	if err := db.Create(tenantOne).Error; err != nil {
		t.Fatalf("failed to seed tenant one: %v", err)
	}
	if err := db.Create(tenantTwo).Error; err != nil {
		t.Fatalf("failed to seed tenant two: %v", err)
	}

	user := &model.RAGUser{
		ID:           7,
		TenantID:     1,
		Email:        "owner@example.com",
		PasswordHash: "hash",
		Name:         "Owner",
		Role:         "owner",
		Status:       "active",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	kbs := []*model.KBKnowledgeBase{
		{TenantID: 1, UserID: 7, Name: "KB One", Description: "one", Status: model.KBKnowledgeBaseStatusActive},
		{TenantID: 1, UserID: 7, Name: "KB Two", Description: "two", Status: model.KBKnowledgeBaseStatusActive},
		{TenantID: 2, UserID: 8, Name: "Other KB", Description: "other", Status: model.KBKnowledgeBaseStatusActive},
	}
	for _, kb := range kbs {
		if err := db.Create(kb).Error; err != nil {
			t.Fatalf("failed to seed knowledge base: %v", err)
		}
	}

	docs := []*model.KBDocument{
		{TenantID: 1, KbID: 1, UserID: 7, FileName: "a.pdf", FileType: "pdf", FileSize: 1572864, FileHash: "hash-a", StoragePath: "/tmp/a.pdf", Status: model.KBDocumentStatusCompleted},
		{TenantID: 1, KbID: 2, UserID: 7, FileName: "b.pdf", FileType: "pdf", FileSize: 2097152, FileHash: "hash-b", StoragePath: "/tmp/b.pdf", Status: model.KBDocumentStatusCompleted},
		{TenantID: 1, KbID: 2, UserID: 7, FileName: "deleted.pdf", FileType: "pdf", FileSize: 9437184, FileHash: "hash-deleted", StoragePath: "/tmp/deleted.pdf", Status: model.KBDocumentStatusCompleted, Deleted: 1},
		{TenantID: 2, KbID: 3, UserID: 8, FileName: "other.pdf", FileType: "pdf", FileSize: 5242880, FileHash: "hash-other", StoragePath: "/tmp/other.pdf", Status: model.KBDocumentStatusCompleted},
	}
	for _, doc := range docs {
		if err := db.Create(doc).Error; err != nil {
			t.Fatalf("failed to seed document: %v", err)
		}
	}
}

func newTenantHandlerTestServer() *server.Hertz {
	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		identity := &authpkg.Identity{
			AuthType: authpkg.AuthTypeJWT,
			TenantID: 1,
			UserID:   7,
			Role:     "owner",
		}
		c.Next(authpkg.WithIdentity(ctx, identity))
	})
	h.GET("/v1/tenant", GetTenant)
	h.GET("/v1/tenant/usage", GetTenantUsage)
	return h
}

func decodeTenantJSONResponse(t *testing.T, body []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, string(body))
	}
}
