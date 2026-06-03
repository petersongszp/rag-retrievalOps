package model

import (
	"testing"
)

func TestParseRetrieveResultStatus(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expected   RetrieveResultStatus
		expectedOk bool
	}{
		{
			name:       "success",
			input:      "success",
			expected:   RetrieveResultStatusSuccess,
			expectedOk: true,
		},
		{
			name:       "no_result",
			input:      "no_result",
			expected:   RetrieveResultStatusNoResult,
			expectedOk: true,
		},
		{
			name:       "filtered_out",
			input:      "filtered_out",
			expected:   RetrieveResultStatusFilteredOut,
			expectedOk: true,
		},
		{
			name:       "error",
			input:      "error",
			expected:   RetrieveResultStatusError,
			expectedOk: true,
		},
		{
			name:       "timeout",
			input:      "timeout",
			expected:   RetrieveResultStatusTimeout,
			expectedOk: true,
		},
		{
			name:       "invalid status",
			input:      "invalid",
			expected:   RetrieveResultStatus(""),
			expectedOk: false,
		},
		{
			name:       "empty string",
			input:      "",
			expected:   RetrieveResultStatus(""),
			expectedOk: false,
		},
		{
			name:       "case sensitive - uppercase",
			input:      "SUCCESS",
			expected:   RetrieveResultStatus(""),
			expectedOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseRetrieveResultStatus(tt.input)
			if ok != tt.expectedOk {
				t.Errorf("ParseRetrieveResultStatus(%q) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}
			if result != tt.expected {
				t.Errorf("ParseRetrieveResultStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRetrieveResultStatusConstants(t *testing.T) {
	if RetrieveResultStatusSuccess != "success" {
		t.Errorf("RetrieveResultStatusSuccess = %q, want %q", RetrieveResultStatusSuccess, "success")
	}
	if RetrieveResultStatusNoResult != "no_result" {
		t.Errorf("RetrieveResultStatusNoResult = %q, want %q", RetrieveResultStatusNoResult, "no_result")
	}
	if RetrieveResultStatusFilteredOut != "filtered_out" {
		t.Errorf("RetrieveResultStatusFilteredOut = %q, want %q", RetrieveResultStatusFilteredOut, "filtered_out")
	}
	if RetrieveResultStatusError != "error" {
		t.Errorf("RetrieveResultStatusError = %q, want %q", RetrieveResultStatusError, "error")
	}
	if RetrieveResultStatusTimeout != "timeout" {
		t.Errorf("RetrieveResultStatusTimeout = %q, want %q", RetrieveResultStatusTimeout, "timeout")
	}
}

func TestKBRetrieveLogTableName(t *testing.T) {
	log := KBRetrieveLog{}
	if log.TableName() != "kb_retrieve_log" {
		t.Errorf("TableName() = %q, want %q", log.TableName(), "kb_retrieve_log")
	}
}

func TestKBRetrieveLogFieldCompleteness(t *testing.T) {
	log := KBRetrieveLog{
		RequestID:              "test-req-001",
		ExperimentID:           "exp-001",
		ExperimentGroup:        "candidate",
		StrategyVersion:        "p3-baseline-v1",
		IndexVersion:           "idx-v1",
		CollectionVersion:      "col-v1",
		CostTraceID:            "cost-001",
		AuditTraceID:           "audit-001",
		ReleaseID:              "release-001",
		UserID:                 1,
		KBIDs:                  "1,2,3",
		Query:                  "Go语言特点",
		Expr:                   `metadata["user_id"] == 1`,
		TopK:                   5,
		ContextTokens:          256,
		QueryType:              "entity",
		Rewrite:                "",
		Routes:                 "dense",
		Collection:             "knowledge",
		RetrieverVersion:       "v1",
		EvidenceGateResult:     "pass",
		CitationSupported:      true,
		CitationSupportScore:   1,
		UnsupportedClaimCount:  0,
		CitationCheckVersion:   "phase3-citation-v1",
		CitationCheckLatencyMs: 12,
		FinalCount:             3,
		TruncatedCount:         0,
		ResultStatus:           RetrieveResultStatusSuccess,
		ErrorCode:              "",
		ErrorMsg:               "",
		EmbeddingMs:            50,
		SearchMs:               120,
		PostprocessMs:          10,
		DurationMs:             180,
		TimeoutMs:              3000,
		TenantID:               7,
		AppID:                  "support-bot",
		APIKeyID:               9,
		AuthType:               "api_key",
		SourceAPI:              "v1",
		PermissionResult:       "allowed",
		IsLegacy:               false,
	}

	if log.RequestID != "test-req-001" {
		t.Errorf("RequestID = %q, want %q", log.RequestID, "test-req-001")
	}
	if log.ExperimentID != "exp-001" || log.ExperimentGroup != "candidate" || log.StrategyVersion != "p3-baseline-v1" {
		t.Errorf("unexpected governance trace fields: %+v", log)
	}
	if log.RetrieverVersion != "v1" {
		t.Errorf("RetrieverVersion = %q, want %q", log.RetrieverVersion, "v1")
	}
	if log.EvidenceGateResult != "pass" {
		t.Errorf("EvidenceGateResult = %q, want %q", log.EvidenceGateResult, "pass")
	}
	if !log.CitationSupported {
		t.Errorf("CitationSupported = %v, want true", log.CitationSupported)
	}
	if log.CitationSupportScore != 1 {
		t.Errorf("CitationSupportScore = %v, want 1", log.CitationSupportScore)
	}
	if log.CitationCheckVersion != "phase3-citation-v1" {
		t.Errorf("CitationCheckVersion = %q, want %q", log.CitationCheckVersion, "phase3-citation-v1")
	}
	if log.CitationCheckLatencyMs != 12 {
		t.Errorf("CitationCheckLatencyMs = %d, want 12", log.CitationCheckLatencyMs)
	}
	if log.EmbeddingMs != 50 {
		t.Errorf("EmbeddingMs = %d, want 50", log.EmbeddingMs)
	}
	if log.ContextTokens != 256 {
		t.Errorf("ContextTokens = %d, want 256", log.ContextTokens)
	}
	if log.SearchMs != 120 {
		t.Errorf("SearchMs = %d, want 120", log.SearchMs)
	}
	if log.PostprocessMs != 10 {
		t.Errorf("PostprocessMs = %d, want 10", log.PostprocessMs)
	}
	if log.DurationMs != 180 {
		t.Errorf("DurationMs = %d, want 180", log.DurationMs)
	}
	if log.TenantID != 7 || log.AppID != "support-bot" || log.APIKeyID != 9 {
		t.Errorf("unexpected platform fields: %+v", log)
	}
	if log.AuthType != "api_key" || log.SourceAPI != "v1" || log.PermissionResult != "allowed" {
		t.Errorf("unexpected auth trace fields: %+v", log)
	}
	if log.ResultStatus != RetrieveResultStatusSuccess {
		t.Errorf("ResultStatus = %q, want %q", log.ResultStatus, RetrieveResultStatusSuccess)
	}
}

func TestKBRetrieveLogResultStatusScenarios(t *testing.T) {
	scenarios := []struct {
		name         string
		resultStatus RetrieveResultStatus
		finalCount   int
		hitCount     int
	}{
		{
			name:         "success with results",
			resultStatus: RetrieveResultStatusSuccess,
			finalCount:   3,
			hitCount:     3,
		},
		{
			name:         "no_result - empty collection",
			resultStatus: RetrieveResultStatusNoResult,
			finalCount:   0,
			hitCount:     0,
		},
		{
			name:         "filtered_out - results exist but all filtered",
			resultStatus: RetrieveResultStatusFilteredOut,
			finalCount:   0,
			hitCount:     5,
		},
		{
			name:         "error - service failure",
			resultStatus: RetrieveResultStatusError,
			finalCount:   0,
			hitCount:     0,
		},
		{
			name:         "timeout - request timeout",
			resultStatus: RetrieveResultStatusTimeout,
			finalCount:   0,
			hitCount:     0,
		},
	}

	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			log := KBRetrieveLog{
				RequestID:    "test-" + tt.name,
				ResultStatus: tt.resultStatus,
				FinalCount:   tt.finalCount,
			}

			parsed, ok := ParseRetrieveResultStatus(string(log.ResultStatus))
			if !ok {
				t.Errorf("ParseRetrieveResultStatus(%q) returned ok=false", log.ResultStatus)
			}
			if parsed != tt.resultStatus {
				t.Errorf("parsed = %q, want %q", parsed, tt.resultStatus)
			}
		})
	}
}
