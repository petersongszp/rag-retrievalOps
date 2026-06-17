package kb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"interview-agents/api/response"
	myerrors "interview-agents/internal/errors"
	"interview-agents/internal/middleware"
	"interview-agents/internal/milvus/evaluation"
	"interview-agents/internal/model"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

type createEvalDatasetRequest struct {
	Name        string `json:"name" vd:"len($)>0"`
	Description string `json:"description"`
	KBID        uint64 `json:"kb_id"`
}

type createEvalCaseRequest struct {
	CaseKey         string                      `json:"case_key" vd:"len($)>0"`
	Query           string                      `json:"query" vd:"len($)>0"`
	TopK            int                         `json:"top_k"`
	RelevantIDs     []string                    `json:"relevant_ids"`
	CitationTargets []evaluation.CitationTarget `json:"citation_targets"`
	QueryType       string                      `json:"query_type"`
	Tags            []string                    `json:"tags"`
	KBIDs           []uint64                    `json:"kb_ids"`
	Collection      string                      `json:"collection"`
	Notes           string                      `json:"notes"`
}

type evalDatasetListResponse struct {
	Items    []*model.KBEvalDataset `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type evalCaseListResponse struct {
	Items    []*model.KBEvalCase `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type evalCaseImportError struct {
	Index   int    `json:"index"`
	CaseKey string `json:"case_key,omitempty"`
	Message string `json:"message"`
}

type evalCaseImportResponse struct {
	Imported int                   `json:"imported"`
	Failed   int                   `json:"failed"`
	Errors   []evalCaseImportError `json:"errors"`
}

type evalDatasetValidationIssue struct {
	CaseID  uint64   `json:"case_id"`
	CaseKey string   `json:"case_key"`
	Errors  []string `json:"errors"`
}

type evalDatasetValidationResponse struct {
	DatasetID      uint64                       `json:"dataset_id"`
	Status         model.KBEvalDatasetStatus    `json:"status"`
	CaseCount      int                          `json:"case_count"`
	ValidCount     int                          `json:"valid_count"`
	InvalidCount   int                          `json:"invalid_count"`
	UncheckedCount int                          `json:"unchecked_count"`
	Issues         []evalDatasetValidationIssue `json:"issues"`
}

func ListEvalDatasets(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	page, pageSize := getPagination(c)
	filter := model.KBEvalDatasetListFilter{
		Page:     page,
		PageSize: pageSize,
	}

	if rawKBID := strings.TrimSpace(string(c.Query("kb_id"))); rawKBID != "" {
		kbID, err := parseUint64(rawKBID, "kb_id")
		if err != nil {
			response.BadRequest(ctx, c, err.Error())
			return
		}
		filter.KBID = &kbID
	}

	if rawStatus := strings.TrimSpace(string(c.Query("status"))); rawStatus != "" {
		status, ok := model.ParseKBEvalDatasetStatus(rawStatus)
		if !ok {
			response.BadRequest(ctx, c, "invalid dataset status")
			return
		}
		filter.Status = &status
	}

	if keyword := strings.TrimSpace(string(c.Query("keyword"))); keyword != "" {
		filter.Keyword = &keyword
	}

	items, total, err := model.KBEvalDatasetDao.ListWithFilter(filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list evaluation datasets", err))
		return
	}

	response.Success(ctx, c, evalDatasetListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func CreateEvalDataset(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	var req createEvalDatasetRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		response.BadRequest(ctx, c, "name is required")
		return
	}

	var kbID *uint64
	if req.KBID > 0 {
		if _, err := mustKnowledgeBaseExist(req.KBID); err != nil {
			response.ErrorFromErr(ctx, c, err)
			return
		}
		kbID = &req.KBID
	}

	dataset := &model.KBEvalDataset{
		Name:        req.Name,
		Description: req.Description,
		KbID:        kbID,
		CaseCount:   0,
		Status:      model.KBEvalDatasetStatusDraft,
		CreatedBy:   middleware.GetUserID(c),
	}
	if err := model.KBEvalDatasetDao.Create(dataset); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to create evaluation dataset", err))
		return
	}

	response.Success(ctx, c, dataset)
}

func ListEvalCases(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	datasetID, err := parseDatasetIDParam(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	if _, err := mustEvalDatasetExist(datasetID); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	page, pageSize := getPagination(c)
	filter := model.KBEvalCaseListFilter{
		DatasetID: datasetID,
		Page:      page,
		PageSize:  pageSize,
	}

	if raw := strings.TrimSpace(string(c.Query("query_type"))); raw != "" {
		filter.QueryType = &raw
	}
	if raw := strings.TrimSpace(string(c.Query("tag"))); raw != "" {
		filter.Tag = &raw
	}
	if raw := strings.TrimSpace(string(c.Query("keyword"))); raw != "" {
		filter.Keyword = &raw
	}
	if raw := strings.TrimSpace(string(c.Query("validation_status"))); raw != "" {
		status, ok := model.ParseKBEvalCaseValidationStatus(raw)
		if !ok {
			response.BadRequest(ctx, c, "invalid validation status")
			return
		}
		filter.ValidationStatus = &status
	}

	items, total, err := model.KBEvalCaseDao.ListWithFilter(filter)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to list evaluation cases", err))
		return
	}

	response.Success(ctx, c, evalCaseListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func CreateEvalCase(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	datasetID, err := parseDatasetIDParam(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	dataset, err := mustEvalDatasetExist(datasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	var req createEvalCaseRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BadRequest(ctx, c, "invalid request: "+err.Error())
		return
	}

	evalCase, err := buildEvalCaseModel(dataset, req)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	exists, err := model.KBEvalCaseDao.ExistsByDatasetIDAndCaseKey(datasetID, evalCase.CaseKey)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to check duplicate evaluation case", err))
		return
	}
	if exists {
		response.BadRequest(ctx, c, "case_key already exists in dataset")
		return
	}

	if err := model.KBEvalCaseDao.Create(evalCase); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to create evaluation case", err))
		return
	}
	if err := syncEvalDatasetCaseCount(datasetID); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	response.Success(ctx, c, evalCase)
}

func ImportEvalCases(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	datasetID, err := parseDatasetIDParam(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	dataset, err := mustEvalDatasetExist(datasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	importedCases, err := readEvalCaseImportPayload(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}

	seenInPayload := make(map[string]struct{}, len(importedCases))
	result := evalCaseImportResponse{Errors: make([]evalCaseImportError, 0)}

	for index, item := range importedCases {
		request := createEvalCaseRequest{
			CaseKey:         strings.TrimSpace(item.ID),
			Query:           item.Query,
			TopK:            item.TopK,
			RelevantIDs:     item.RelevantIDs,
			CitationTargets: item.CitationTargets,
			QueryType:       item.QueryType,
			Tags:            item.Tags,
			KBIDs:           item.KBIDs,
			Collection:      item.Collection,
			Notes:           item.Notes,
		}
		evalCase, buildErr := buildEvalCaseModel(dataset, request)
		if buildErr != nil {
			result.Errors = append(result.Errors, evalCaseImportError{
				Index:   index,
				CaseKey: strings.TrimSpace(item.ID),
				Message: buildErr.Error(),
			})
			continue
		}
		if _, exists := seenInPayload[evalCase.CaseKey]; exists {
			result.Errors = append(result.Errors, evalCaseImportError{
				Index:   index,
				CaseKey: evalCase.CaseKey,
				Message: "duplicate case_key in import payload",
			})
			continue
		}
		seenInPayload[evalCase.CaseKey] = struct{}{}

		exists, existsErr := model.KBEvalCaseDao.ExistsByDatasetIDAndCaseKey(datasetID, evalCase.CaseKey)
		if existsErr != nil {
			response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to check duplicate evaluation case", existsErr))
			return
		}
		if exists {
			result.Errors = append(result.Errors, evalCaseImportError{
				Index:   index,
				CaseKey: evalCase.CaseKey,
				Message: "case_key already exists in dataset",
			})
			continue
		}

		if err := model.KBEvalCaseDao.Create(evalCase); err != nil {
			result.Errors = append(result.Errors, evalCaseImportError{
				Index:   index,
				CaseKey: evalCase.CaseKey,
				Message: err.Error(),
			})
			continue
		}
		result.Imported++
	}

	result.Failed = len(result.Errors)
	if err := syncEvalDatasetCaseCount(datasetID); err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

func ExportEvalCases(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	datasetID, err := parseDatasetIDParam(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	dataset, err := mustEvalDatasetExist(datasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	items, err := model.KBEvalCaseDao.ListAllByDatasetID(datasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to export evaluation cases", err))
		return
	}

	exported := make([]evaluation.DatasetCase, 0, len(items))
	for _, item := range items {
		exported = append(exported, model.KBEvalCaseDao.ToDatasetCase(item))
	}

	fileName := sanitizeEvalDatasetFileName(dataset.Name)
	if fileName == "" {
		fileName = fmt.Sprintf("eval-dataset-%d", dataset.ID)
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", fileName))
	c.JSON(consts.StatusOK, exported)
}

func ValidateEvalDataset(ctx context.Context, c *app.RequestContext) {
	if !requireAdmin(ctx, c) {
		return
	}

	datasetID, err := parseDatasetIDParam(c)
	if err != nil {
		response.BadRequest(ctx, c, err.Error())
		return
	}
	dataset, err := mustEvalDatasetExist(datasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, err)
		return
	}

	items, err := model.KBEvalCaseDao.ListAllByDatasetID(datasetID)
	if err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to load evaluation cases", err))
		return
	}

	caseKeyCounts := make(map[string]int, len(items))
	for _, item := range items {
		caseKeyCounts[item.CaseKey]++
	}

	result := evalDatasetValidationResponse{
		DatasetID: datasetID,
		CaseCount: len(items),
		Issues:    make([]evalDatasetValidationIssue, 0),
	}

	if err := model.WithTransaction(ctx, func(tx *gorm.DB) error {
		for _, item := range items {
			validationErrors := validateEvalCaseAgainstDataset(item, dataset, caseKeyCounts)
			status := model.KBEvalCaseValidationStatusValid
			if len(validationErrors) > 0 {
				status = model.KBEvalCaseValidationStatusInvalid
				result.InvalidCount++
				result.Issues = append(result.Issues, evalDatasetValidationIssue{
					CaseID:  item.ID,
					CaseKey: item.CaseKey,
					Errors:  validationErrors,
				})
			} else {
				result.ValidCount++
			}

			if err := tx.Model(&model.KBEvalCase{}).
				Where("id = ?", item.ID).
				Updates(map[string]interface{}{
					"validation_status": status,
					"validation_errors": model.StringList(validationErrors),
				}).Error; err != nil {
				return err
			}
		}

		nextStatus := model.KBEvalDatasetStatusDraft
		switch {
		case len(items) == 0:
			nextStatus = model.KBEvalDatasetStatusDraft
		case result.InvalidCount > 0:
			nextStatus = model.KBEvalDatasetStatusInvalid
		default:
			nextStatus = model.KBEvalDatasetStatusReady
		}
		result.Status = nextStatus

		return tx.Model(&model.KBEvalDataset{}).
			Where("id = ?", datasetID).
			Updates(map[string]interface{}{
				"status":     nextStatus,
				"case_count": len(items),
			}).Error
	}); err != nil {
		response.ErrorFromErr(ctx, c, myerrors.NewDBError("failed to validate evaluation dataset", err))
		return
	}

	response.Success(ctx, c, result)
}

func mustEvalDatasetExist(datasetID uint64) (*model.KBEvalDataset, error) {
	dataset, err := model.KBEvalDatasetDao.GetByID(datasetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, myerrors.NewNotFoundError("evaluation dataset")
		}
		return nil, myerrors.NewDBError("failed to get evaluation dataset", err)
	}
	return dataset, nil
}

func parseDatasetIDParam(c *app.RequestContext) (uint64, error) {
	return parseUint64(strings.TrimSpace(c.Param("dataset_id")), "dataset_id")
}

func buildEvalCaseModel(dataset *model.KBEvalDataset, req createEvalCaseRequest) (*model.KBEvalCase, error) {
	caseKey := strings.TrimSpace(req.CaseKey)
	query := strings.TrimSpace(req.Query)
	queryType := strings.TrimSpace(req.QueryType)
	collection := strings.TrimSpace(req.Collection)
	notes := strings.TrimSpace(req.Notes)
	if caseKey == "" {
		return nil, fmt.Errorf("case_key is required")
	}
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.TopK <= 0 {
		return nil, fmt.Errorf("top_k must be greater than 0")
	}

	evalCase := &model.KBEvalCase{
		DatasetID:        dataset.ID,
		CaseKey:          caseKey,
		Query:            query,
		TopK:             req.TopK,
		RelevantIDs:      normalizeStringList(req.RelevantIDs),
		CitationTargets:  model.CitationTargetList(req.CitationTargets),
		QueryType:        queryType,
		Tags:             normalizeStringList(req.Tags),
		KBIDs:            model.Uint64List(req.KBIDs),
		Collection:       collection,
		Notes:            notes,
		ValidationStatus: model.KBEvalCaseValidationStatusUnchecked,
		ValidationErrors: model.StringList{},
	}
	return evalCase, nil
}

func normalizeStringList(items []string) model.StringList {
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return model.StringList(result)
}

func readEvalCaseImportPayload(c *app.RequestContext) ([]evaluation.DatasetCase, error) {
	if fileHeader, err := c.FormFile("file"); err == nil && fileHeader != nil {
		content, readErr := readMultipartFile(fileHeader)
		if readErr != nil {
			return nil, readErr
		}
		return decodeEvalCaseImportPayload(content)
	}

	body := c.GetRequest().Body()
	if len(body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}
	return decodeEvalCaseImportPayload(body)
}

func decodeEvalCaseImportPayload(content []byte) ([]evaluation.DatasetCase, error) {
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return nil, fmt.Errorf("request body is empty")
	}

	var items []evaluation.DatasetCase
	if err := json.Unmarshal([]byte(trimmed), &items); err == nil {
		return items, nil
	}

	var bundle evaluation.DatasetBundle
	if err := json.Unmarshal([]byte(trimmed), &bundle); err != nil {
		return nil, fmt.Errorf("invalid import JSON: %w", err)
	}
	return bundle.Cases, nil
}

func readMultipartFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open import file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read import file: %w", err)
	}
	return content, nil
}

func syncEvalDatasetCaseCount(datasetID uint64) error {
	count, err := model.KBEvalCaseDao.CountByDatasetID(datasetID)
	if err != nil {
		return myerrors.NewDBError("failed to count evaluation cases", err)
	}
	return model.KBEvalDatasetDao.UpdateCaseCount(datasetID, int(count))
}

func validateEvalCaseAgainstDataset(evalCase *model.KBEvalCase, dataset *model.KBEvalDataset, caseKeyCounts map[string]int) []string {
	errorsList := make([]string, 0)
	if strings.TrimSpace(evalCase.Query) == "" {
		errorsList = append(errorsList, "query is required")
	}
	if evalCase.TopK <= 0 {
		errorsList = append(errorsList, "top_k must be greater than 0")
	}
	if len(evalCase.RelevantIDs) == 0 && len(evalCase.CitationTargets) == 0 {
		errorsList = append(errorsList, "relevant_ids or citation_targets is required")
	}
	if caseKeyCounts[evalCase.CaseKey] > 1 {
		errorsList = append(errorsList, "case_key must be unique within dataset")
	}
	if dataset.KbID != nil && len(evalCase.KBIDs) > 0 && !evalCase.KBIDs.Contains(*dataset.KbID) {
		errorsList = append(errorsList, "kb_ids must include dataset kb_id when dataset is bound to a knowledge base")
	}
	return errorsList
}

func sanitizeEvalDatasetFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "\"", "")
	return name
}
