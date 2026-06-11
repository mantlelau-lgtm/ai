package metrics

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type Registry struct {
	inboundRequests atomic.Uint64
	inboundEvents   atomic.Uint64
	duplicateEvents atomic.Uint64
	jobsCreated     atomic.Uint64
	jobsSucceeded   atomic.Uint64
	jobsRetried     atomic.Uint64
	jobsDead        atomic.Uint64
	outboundSent    atomic.Uint64
}

func New() *Registry {
	return &Registry{}
}

func (r *Registry) IncInboundRequests() { r.inboundRequests.Add(1) }
func (r *Registry) IncInboundEvents()   { r.inboundEvents.Add(1) }
func (r *Registry) IncDuplicateEvents() { r.duplicateEvents.Add(1) }
func (r *Registry) IncJobsCreated()     { r.jobsCreated.Add(1) }
func (r *Registry) IncJobsSucceeded()   { r.jobsSucceeded.Add(1) }
func (r *Registry) IncJobsRetried()     { r.jobsRetried.Add(1) }
func (r *Registry) IncJobsDead()        { r.jobsDead.Add(1) }
func (r *Registry) IncOutboundSent()    { r.outboundSent.Add(1) }

func (r *Registry) RenderPrometheus() string {
	lines := []string{
		"# HELP mgw_inbound_requests_total Total HTTP callback requests.",
		"# TYPE mgw_inbound_requests_total counter",
		fmt.Sprintf("mgw_inbound_requests_total %d", r.inboundRequests.Load()),
		"# HELP mgw_inbound_events_total Total accepted inbound events.",
		"# TYPE mgw_inbound_events_total counter",
		fmt.Sprintf("mgw_inbound_events_total %d", r.inboundEvents.Load()),
		"# HELP mgw_duplicate_events_total Total duplicate inbound events.",
		"# TYPE mgw_duplicate_events_total counter",
		fmt.Sprintf("mgw_duplicate_events_total %d", r.duplicateEvents.Load()),
		"# HELP mgw_jobs_created_total Total jobs created.",
		"# TYPE mgw_jobs_created_total counter",
		fmt.Sprintf("mgw_jobs_created_total %d", r.jobsCreated.Load()),
		"# HELP mgw_jobs_succeeded_total Total successful jobs.",
		"# TYPE mgw_jobs_succeeded_total counter",
		fmt.Sprintf("mgw_jobs_succeeded_total %d", r.jobsSucceeded.Load()),
		"# HELP mgw_jobs_retried_total Total retried jobs.",
		"# TYPE mgw_jobs_retried_total counter",
		fmt.Sprintf("mgw_jobs_retried_total %d", r.jobsRetried.Load()),
		"# HELP mgw_jobs_dead_total Total dead-letter jobs.",
		"# TYPE mgw_jobs_dead_total counter",
		fmt.Sprintf("mgw_jobs_dead_total %d", r.jobsDead.Load()),
		"# HELP mgw_outbound_sent_total Total outbound messages sent to Lark.",
		"# TYPE mgw_outbound_sent_total counter",
		fmt.Sprintf("mgw_outbound_sent_total %d", r.outboundSent.Load()),
	}

	return strings.Join(lines, "\n") + "\n"
}
