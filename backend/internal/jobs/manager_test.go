package jobs

import "testing"

func TestRegisterJobIfNoDuplicateRejectsMatchingActiveJob(t *testing.T) {
	m := NewManager(1)
	first := &Job{ID: "j1", Type: JobTypeScan, Subtype: "trivy", Target: "nginx:latest", Status: "queued"}
	if existing, dup := m.RegisterJobIfNoDuplicate(first); dup || existing != "" {
		t.Fatalf("expected first registration to succeed, got dup=%v existing=%q", dup, existing)
	}

	second := &Job{ID: "j2", Type: JobTypeScan, Subtype: "trivy", Target: "nginx:latest", Status: "queued"}
	existing, dup := m.RegisterJobIfNoDuplicate(second)
	if !dup {
		t.Fatalf("expected duplicate registration to be rejected")
	}
	if existing != "j1" {
		t.Fatalf("expected duplicate to point at j1, got %q", existing)
	}
}

func TestRegisterJobIfNoDuplicateAllowsDifferentSubtypeOrTarget(t *testing.T) {
	m := NewManager(1)
	_, _ = m.RegisterJobIfNoDuplicate(&Job{ID: "j1", Type: JobTypeScan, Subtype: "trivy", Target: "nginx:latest", Status: "queued"})
	if _, dup := m.RegisterJobIfNoDuplicate(&Job{ID: "j2", Type: JobTypeScan, Subtype: "clamav", Target: "nginx:latest", Status: "queued"}); dup {
		t.Fatalf("expected different subtype to be allowed")
	}
	if _, dup := m.RegisterJobIfNoDuplicate(&Job{ID: "j3", Type: JobTypeScan, Subtype: "trivy", Target: "redis:latest", Status: "queued"}); dup {
		t.Fatalf("expected different target to be allowed")
	}
}
