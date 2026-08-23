package executord

import (
	"sync"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/pipeline"
	"github.com/FlameInTheDark/neuropipe/internal/scheduler"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// CronScheduler autonomously fires trusted, enabled cron triggers from
// deployed bundles. It mirrors the desktop scheduler semantics (five-field
// expressions, optional CRON_TZ timezone) without depending on SQLite.
type CronScheduler struct {
	store   *store
	enqueue func(job runnerJob)

	mu      sync.Mutex
	entries map[string]cron.EntryID
	cron    *cron.Cron
}

// NewCronScheduler wires the scheduler to a deployment store. The enqueue
// callback starts one queued run per fire.
func NewCronScheduler(store *store, runner *Runner) *CronScheduler {
	return &CronScheduler{
		store:   store,
		enqueue: func(job runnerJob) { _ = runner.enqueueJob(job) },
		entries: make(map[string]cron.EntryID),
		cron:    cron.New(),
	}
}

// Start begins the cron worker and loads current entries.
func (s *CronScheduler) Start() error {
	s.cron.Start()
	return s.Reload()
}

// Stop removes every entry and terminates the worker.
func (s *CronScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		s.cron.Remove(entry)
	}
	s.entries = make(map[string]cron.EntryID)
	<-s.cron.Stop().Done()
}

// Reload reconciles active cron jobs against deployed bundles.
func (s *CronScheduler) Reload() error {
	bundles := s.store.ListBundles()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		s.cron.Remove(entry)
	}
	s.entries = make(map[string]cron.EntryID)
	for _, bundle := range bundles {
		for _, trigger := range bundle.Triggers {
			if trigger.Kind != string(domain.TriggerCron) || !trigger.Enabled || !trigger.Trusted {
				continue
			}
			spec, err := scheduler.CronSpec(trigger.Cron, trigger.Timezone)
			if err != nil {
				continue
			}
			target := bundle
			snapshot := trigger
			entry, err := s.cron.AddFunc(spec, func() { fireScheduled(&target, snapshot, s.enqueue) })
			if err != nil {
				continue
			}
			s.entries[trigger.BindingID] = entry
		}
	}
	return nil
}

// fireScheduled queues one unattended run for an autonomous schedule fire.
func fireScheduled(bundle *DeployedPipeline, trigger DeployedTriggerSnapshot, enqueue func(runnerJob)) {
	now := time.Now().UTC()
	record := RunRecord{
		ExecutionID:      uuid.NewString(),
		PipelineID:       bundle.PipelineID,
		Name:             bundle.Name,
		Revision:         bundle.Revision,
		TriggerNodeID:    trigger.NodeID,
		TriggerBindingID: trigger.BindingID,
		Status:           domain.RunPending,
		StartedAt:        now,
	}
	job := runnerJob{
		record:        record,
		definition:    bundle.Definition,
		triggerNodeID: trigger.NodeID,
		input:         pipeline.Packet{"trigger": "cron", "scheduledAt": now.Format(time.RFC3339)},
		unattended:    true,
		capabilities:  bundle.Capabilities,
	}
	enqueue(job)
}
