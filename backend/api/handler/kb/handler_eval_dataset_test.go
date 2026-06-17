package kb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol"
	mysqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const defaultEvalTestMySQLAdminDSN = "root:root@tcp(127.0.0.1:3307)/mysql?charset=utf8mb4&parseTime=True&loc=Local"

func TestEvalDatasetL1Flow(t *testing.T) {
	db := setupEvalDatasetTestDB(t)
	h := newAdminEvalDatasetTestServer()

	kbRecord := &model.KBKnowledgeBase{
		UserID:      7,
		Name:        "phase2-test-kb",
		Description: "test kb",
		Status:      model.KBKnowledgeBaseStatusActive,
	}
	if err := db.Create(kbRecord).Error; err != nil {
		t.Fatalf("failed to seed knowledge base: %v", err)
	}

	createDatasetResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/datasets", map[string]interface{}{
		"name":        "phase2-core-regression",
		"description": "core regression dataset",
		"kb_id":       kbRecord.ID,
	})
	if createDatasetResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected create dataset 200, got %d", createDatasetResp.StatusCode())
	}

	var datasetPayload struct {
		Code int                 `json:"code"`
		Data model.KBEvalDataset `json:"data"`
	}
	decodeJSONResponse(t, createDatasetResp.Body(), &datasetPayload)
	if datasetPayload.Code != 200 || datasetPayload.Data.ID == 0 {
		t.Fatalf("unexpected dataset payload: %+v", datasetPayload)
	}

	createCaseResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/datasets/"+toString(datasetPayload.Data.ID)+"/items", map[string]interface{}{
		"case_key":     "manual-valid-case",
		"query":        "golang channel close panic",
		"top_k":        5,
		"relevant_ids": []string{"chunk-1"},
		"query_type":   "entity",
		"tags":         []string{"golang", "channel"},
	})
	if createCaseResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected create case 200, got %d", createCaseResp.StatusCode())
	}

	importFile := filepath.Join("..", "..", "..", "scripts", "evaluation", "dataset.json")
	importBody, err := os.ReadFile(importFile)
	if err != nil {
		t.Fatalf("failed to read import fixture: %v", err)
	}

	importResp := ut.PerformRequest(
		h.Engine,
		http.MethodPost,
		"/api/admin/kb/eval/datasets/"+toString(datasetPayload.Data.ID)+"/items/import",
		&ut.Body{Body: bytes.NewReader(importBody), Len: len(importBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	).Result()
	if importResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected import 200, got %d", importResp.StatusCode())
	}

	var importPayload struct {
		Code int `json:"code"`
		Data struct {
			Imported int `json:"imported"`
			Failed   int `json:"failed"`
		} `json:"data"`
	}
	decodeJSONResponse(t, importResp.Body(), &importPayload)
	if importPayload.Code != 200 || importPayload.Data.Imported == 0 || importPayload.Data.Failed != 0 {
		t.Fatalf("unexpected import payload: %+v", importPayload)
	}

	invalidCaseResp := performJSONRequest(t, h, http.MethodPost, "/api/admin/kb/eval/datasets/"+toString(datasetPayload.Data.ID)+"/items", map[string]interface{}{
		"case_key": "manual-invalid-case",
		"query":    "missing golden answer",
		"top_k":    5,
	})
	if invalidCaseResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected invalid draft case create 200, got %d", invalidCaseResp.StatusCode())
	}

	validateResp := ut.PerformRequest(
		h.Engine,
		http.MethodPost,
		"/api/admin/kb/eval/datasets/"+toString(datasetPayload.Data.ID)+"/validate",
		nil,
	).Result()
	if validateResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected validate 200, got %d", validateResp.StatusCode())
	}

	var validatePayload struct {
		Code int `json:"code"`
		Data struct {
			Status       string `json:"status"`
			InvalidCount int    `json:"invalid_count"`
			Issues       []struct {
				CaseKey string   `json:"case_key"`
				Errors  []string `json:"errors"`
			} `json:"issues"`
		} `json:"data"`
	}
	decodeJSONResponse(t, validateResp.Body(), &validatePayload)
	if validatePayload.Code != 200 {
		t.Fatalf("unexpected validate response code: %+v", validatePayload)
	}
	if validatePayload.Data.Status != string(model.KBEvalDatasetStatusInvalid) {
		t.Fatalf("expected dataset invalid after validation, got %q", validatePayload.Data.Status)
	}
	if validatePayload.Data.InvalidCount == 0 || len(validatePayload.Data.Issues) == 0 {
		t.Fatalf("expected validation issues, got %+v", validatePayload.Data)
	}

	exportResp := ut.PerformRequest(
		h.Engine,
		http.MethodGet,
		"/api/admin/kb/eval/datasets/"+toString(datasetPayload.Data.ID)+"/items/export",
		nil,
	).Result()
	if exportResp.StatusCode() != http.StatusOK {
		t.Fatalf("expected export 200, got %d", exportResp.StatusCode())
	}

	tempDir := t.TempDir()
	exportPath := filepath.Join(tempDir, "dataset.json")
	if err := os.WriteFile(exportPath, exportResp.Body(), 0o644); err != nil {
		t.Fatalf("failed to write export file: %v", err)
	}
	exportedDataset, err := evaluation.LoadDataset(exportPath)
	if err != nil {
		t.Fatalf("export should be parseable by evaluation.LoadDataset: %v", err)
	}
	if len(exportedDataset) < 2 {
		t.Fatalf("expected exported dataset size >= 2, got %d", len(exportedDataset))
	}
}

func setupEvalDatasetTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	adminDSN := resolveEvalTestMySQLAdminDSN()
	adminDB, err := openMySQLWithRetry(adminDSN)
	if err != nil {
		t.Skipf("skipping eval mysql integration tests: failed to open mysql admin db: %v", err)
	}

	testDBName := fmt.Sprintf("interview_agent_eval_l1_test_%d", time.Now().UnixNano())
	if err := adminDB.Exec("CREATE DATABASE IF NOT EXISTS " + testDBName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() {
		_ = adminDB.Exec("DROP DATABASE IF EXISTS " + testDBName).Error
	})

	db, err := openMySQLWithRetry(buildEvalTestDatabaseDSN(adminDSN, testDBName))
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&model.KBKnowledgeBase{}, &model.KBEvalDataset{}, &model.KBEvalCase{}, &model.KBEvalRun{}, &model.KBRetrieveLog{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	model.SetDBGetter(func() *gorm.DB { return db })
	t.Cleanup(func() {
		model.SetDBGetter(nil)
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newAdminEvalDatasetTestServer() *server.Hertz {
	h := server.Default()
	h.Use(func(ctx context.Context, c *app.RequestContext) {
		c.Set("user_id", uint(7))
		c.Set("role", "admin")
		c.Next(ctx)
	})
	h.GET("/api/admin/kb/eval/datasets", ListEvalDatasets)
	h.POST("/api/admin/kb/eval/datasets", CreateEvalDataset)
	h.GET("/api/admin/kb/eval/datasets/:dataset_id/items", ListEvalCases)
	h.POST("/api/admin/kb/eval/datasets/:dataset_id/items", CreateEvalCase)
	h.POST("/api/admin/kb/eval/datasets/:dataset_id/items/import", ImportEvalCases)
	h.GET("/api/admin/kb/eval/datasets/:dataset_id/items/export", ExportEvalCases)
	h.POST("/api/admin/kb/eval/datasets/:dataset_id/validate", ValidateEvalDataset)
	return h
}

func performJSONRequest(t *testing.T, h *server.Hertz, method string, path string, payload interface{}) *protocol.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return ut.PerformRequest(
		h.Engine,
		method,
		path,
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	).Result()
}

func decodeJSONResponse(t *testing.T, body []byte, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, string(body))
	}
}

func toString(value uint64) string {
	return fmt.Sprintf("%d", value)
}

func openMySQLWithRetry(dsn string) (*gorm.DB, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
		if err == nil {
			sqlDB, sqlErr := db.DB()
			if sqlErr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					return db, nil
				} else {
					lastErr = pingErr
				}
			} else {
				lastErr = sqlErr
			}
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, lastErr
}

func resolveEvalTestMySQLAdminDSN() string {
	for _, key := range []string{"KB_EVAL_TEST_MYSQL_ADMIN_DSN", "KB_TEST_MYSQL_ADMIN_DSN"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return defaultEvalTestMySQLAdminDSN
}

func buildEvalTestDatabaseDSN(adminDSN string, databaseName string) string {
	cfg, err := mysqldriver.ParseDSN(adminDSN)
	if err != nil {
		return adminDSN
	}
	cfg.DBName = databaseName
	return cfg.FormatDSN()
}
