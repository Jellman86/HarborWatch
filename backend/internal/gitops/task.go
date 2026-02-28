package gitops

import (
	"context"
	"fmt"
	"log"
)

type gitopsSyncTask struct {
	service *Service
}

func NewSyncTask(service *Service) *gitopsSyncTask {
	return &gitopsSyncTask{
		service: service,
	}
}

func (t *gitopsSyncTask) Name() string {
	return "gitops_sync"
}

func (t *gitopsSyncTask) Run(ctx context.Context) error {
	sources, err := t.service.store.ListSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to list git sources: %w", err)
	}

	for _, src := range sources {
		// We could implement fine-grained scheduling per source here, 
		// but for the MVP we'll just check if it's time to sync based on LastSyncAt and SyncIntervalMins.
		// Actually, the scheduler itself can run this task every minute, and we decide here.
		
		// For simplicity in MVP, we just sync all of them when the task runs.
		// The user can configure the task itself to run every 5 mins.
		
		log.Printf("[INFO] GitOps: Syncing source %s (%s)", src.Name, src.ID)
		hash, changed, err := t.service.SyncSource(ctx, src.ID)
		if err != nil {
			log.Printf("[ERROR] GitOps: Sync failed for %s: %v", src.Name, err)
			continue
		}

		if changed {
			log.Printf("[INFO] GitOps: Source %s changed to %s, triggering deployments", src.Name, hash)
			deployErrs := t.service.DeployAllForSource(ctx, src.ID)
			for _, de := range deployErrs {
				log.Printf("[ERROR] GitOps: Deployment failed for source %s: %v", src.Name, de)
			}
		}
	}

	return nil
}
