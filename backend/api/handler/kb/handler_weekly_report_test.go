package kb

import "testing"

func TestWeeklyReportResponseShape(t *testing.T) {
	resp := weeklyReportResponse{}
	if len(resp.Risks) != 0 {
		t.Fatalf("expected empty default risks")
	}
}
