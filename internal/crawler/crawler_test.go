package crawler

import "testing"

func TestCandidateScoring(t *testing.T) {
	if got := scoreCandidate("/jobs/backend-engineer", "Junior Backend Engineer", "Da Nang Full-time", nil); got < 6 {
		t.Fatalf("wanted job candidate score >= 6, got %d", got)
	}
	if got := scoreCandidate("/blog/product", "Product update", "", nil); got >= 6 {
		t.Fatalf("non-job link should stay below threshold, got %d", got)
	}
}

func TestSafeURLRejectsPrivateNetworks(t *testing.T) {
	if err := safeURL("http://127.0.0.1/jobs"); err == nil {
		t.Fatal("expected loopback URL to be rejected")
	}
}
