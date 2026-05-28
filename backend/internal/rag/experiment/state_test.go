package experiment

import (
	"testing"
	"time"

	"interview-agents/internal/config"
	"interview-agents/internal/rag/governance"
)

type fakeRetrieveLog struct {
	experimentID    string
	experimentGroup string
	durationMS      int64
	contextTokens   int
	resultStatus    string
}

func (f fakeRetrieveLog) GetExperimentID() string    { return f.experimentID }
func (f fakeRetrieveLog) GetExperimentGroup() string { return f.experimentGroup }
func (f fakeRetrieveLog) GetDurationMS() int64       { return f.durationMS }
func (f fakeRetrieveLog) GetContextTokens() int      { return f.contextTokens }
func (f fakeRetrieveLog) GetResultStatus() string    { return f.resultStatus }

func TestSaveAndDecide(t *testing.T) {
	governance.ResetCompensationTasks()
	record, err := Save(ConfigRecord{
		ExperimentName:    "rewrite shadow",
		StrategyType:      StrategyTypeRewrite,
		BaselineVersion:   "rewrite_on",
		CandidateVersion:  "rewrite_off",
		TrafficRatio:      20,
		TargetEnvironment: EnvAll,
		ShadowMode:        true,
		StartTime:         time.Now().UTC().Add(-time.Hour),
		EndTime:           time.Now().UTC().Add(time.Hour),
		Owner:             "tester",
		Status:            StatusRunning,
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	cfg := &config.Config{
		RAG: config.RAGConfig{
			FeatureFlags: config.RAGFeatureFlags{
				EnableExperimentPlatform: true,
			},
		},
	}
	decision := Decide(cfg, 7, "user", []uint64{1}, "what is go", "req-1", 5)
	if !decision.Matched || decision.Group != GroupShadow {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.ExperimentID != record.ExperimentID {
		t.Fatalf("decision experiment_id = %q, want %q", decision.ExperimentID, record.ExperimentID)
	}
	if !decision.Override.ForceRewriteOff {
		t.Fatalf("expected rewrite experiment override: %+v", decision.Override)
	}
}

func TestCandidateTopKOverride(t *testing.T) {
	_, err := Save(ConfigRecord{
		ExperimentName:    "topk candidate",
		StrategyType:      StrategyTypeCandidateTopK,
		BaselineVersion:   "topk:5",
		CandidateVersion:  "topk:12",
		TrafficRatio:      100,
		TargetEnvironment: EnvAll,
		StartTime:         time.Now().UTC().Add(-time.Hour),
		EndTime:           time.Now().UTC().Add(time.Hour),
		Owner:             "tester",
		Status:            StatusRunning,
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	cfg := &config.Config{
		RAG: config.RAGConfig{
			FeatureFlags: config.RAGFeatureFlags{
				EnableExperimentPlatform: true,
			},
		},
	}
	decision := Decide(cfg, 9, "user", []uint64{1}, "how topk works", "req-2", 5)
	if !decision.Matched || decision.Group != GroupCandidate {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.Override.CandidateTopK != 12 {
		t.Fatalf("candidate_topk = %d, want 12", decision.Override.CandidateTopK)
	}
}

func TestRollbackStopsExperiment(t *testing.T) {
	record, err := Save(ConfigRecord{
		ExperimentName:    "rollback me",
		StrategyType:      StrategyTypeRewrite,
		BaselineVersion:   "rewrite_on",
		CandidateVersion:  "rewrite_off",
		TrafficRatio:      100,
		TargetEnvironment: EnvAll,
		StartTime:         time.Now().UTC().Add(-time.Hour),
		EndTime:           time.Now().UTC().Add(time.Hour),
		Owner:             "tester",
		Status:            StatusRunning,
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	cfg := &config.Config{}
	rolled, err := Rollback(nil, cfg, record.ExperimentID, "manual rollback")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if rolled.Status != StatusStopped || rolled.TrafficRatio != 0 {
		t.Fatalf("unexpected rollback result: %+v", rolled)
	}
}

func TestBuildSummary(t *testing.T) {
	record, err := Save(ConfigRecord{
		ExperimentName:    "summary exp",
		StrategyType:      StrategyTypeCandidateTopK,
		BaselineVersion:   "topk:5",
		CandidateVersion:  "topk:8",
		TrafficRatio:      100,
		TargetEnvironment: EnvAll,
		StartTime:         time.Now().UTC().Add(-time.Hour),
		EndTime:           time.Now().UTC().Add(time.Hour),
		Owner:             "tester",
		Status:            StatusRunning,
	})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	summaries := BuildSummary([]RetrieveLogLike{
		fakeRetrieveLog{experimentID: record.ExperimentID, experimentGroup: GroupBaseline, durationMS: 100, contextTokens: 200, resultStatus: "success"},
		fakeRetrieveLog{experimentID: record.ExperimentID, experimentGroup: GroupCandidate, durationMS: 180, contextTokens: 260, resultStatus: "success"},
	})
	if len(summaries) == 0 {
		t.Fatal("expected summary")
	}
}
