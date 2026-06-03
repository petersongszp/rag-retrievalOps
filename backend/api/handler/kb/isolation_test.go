package kb

import (
	"testing"

	"interview-agents/internal/model"
)

func TestIsKnowledgeBaseAccessibleToActor(t *testing.T) {
	tests := []struct {
		name     string
		kb       *model.KBKnowledgeBase
		tenantID uint64
		userID   uint
		want     bool
	}{
		{
			name: "tenant match",
			kb: &model.KBKnowledgeBase{
				TenantID: 12,
				UserID:   7,
			},
			tenantID: 12,
			userID:   99,
			want:     true,
		},
		{
			name: "tenant mismatch",
			kb: &model.KBKnowledgeBase{
				TenantID: 12,
				UserID:   7,
			},
			tenantID: 18,
			userID:   7,
			want:     false,
		},
		{
			name: "legacy user fallback match",
			kb: &model.KBKnowledgeBase{
				TenantID: 0,
				UserID:   23,
			},
			tenantID: 45,
			userID:   23,
			want:     true,
		},
		{
			name: "legacy user fallback mismatch",
			kb: &model.KBKnowledgeBase{
				TenantID: 0,
				UserID:   23,
			},
			tenantID: 45,
			userID:   24,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnowledgeBaseAccessibleToActor(tt.kb, tt.tenantID, tt.userID); got != tt.want {
				t.Fatalf("isKnowledgeBaseAccessibleToActor() = %v, want %v", got, tt.want)
			}
		})
	}
}
