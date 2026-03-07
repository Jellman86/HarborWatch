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

func TestRegisterJobIfNoDuplicateRejectsMatchingGitOpsDeployTarget(t *testing.T) {
	m := NewManager(1)
	if existing, dup := m.RegisterJobIfNoDuplicate(&Job{ID: "deploy-1", Type: JobTypeGitOpsDeploy, Target: "deployment:dep-1", Status: "queued"}); dup || existing != "" {
		t.Fatalf("expected first gitops deploy registration to succeed, got dup=%v existing=%q", dup, existing)
	}

	existing, dup := m.RegisterJobIfNoDuplicate(&Job{ID: "deploy-2", Type: JobTypeGitOpsDeploy, Target: "deployment:dep-1", Status: "running"})
	if !dup {
		t.Fatalf("expected duplicate gitops deploy registration to be rejected")
	}
	if existing != "deploy-1" {
		t.Fatalf("expected duplicate to point at deploy-1, got %q", existing)
	}
}

func TestActiveJobsReturnsStableSortedOrder(t *testing.T) {
	m := NewManager(2)
	m.RegisterJob(&Job{ID: "z", Type: JobTypeScan, Subtype: "trivy", TargetName: "redis", Status: "queued", StartedAt: 20})
	m.RegisterJob(&Job{ID: "a", Type: JobTypeUpdate, TargetName: "nginx", Status: "running", StartedAt: 10})
	m.RegisterJob(&Job{ID: "b", Type: JobTypeScan, Subtype: "clamav", TargetName: "alpine", Status: "queued", StartedAt: 20})

	got := m.ActiveJobs()
	if len(got) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(got))
	}

	wantIDs := []string{"a", "b", "z"}
	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Fatalf("expected job[%d]=%q, got %q", i, want, got[i].ID)
		}
	}
}
