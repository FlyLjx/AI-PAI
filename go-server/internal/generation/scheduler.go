package generation

import "strings"

// scheduleJobs applies per-key limits before work occupies a worker. It scans
// past blocked scopes so one busy API key cannot stall unrelated customers.
func (q *Queue) scheduleJobs() {
	pending := make([]streamJob, 0, cap(q.incoming))
	active := map[string]int{}
	for {
		for {
			index := nextEligibleJob(pending, active)
			if index < 0 {
				break
			}
			job := pending[index]
			select {
			case q.jobs <- job:
				scope := strings.TrimSpace(job.ConcurrencyScope)
				if scope != "" {
					active[scope]++
				}
				pending = append(pending[:index], pending[index+1:]...)
			default:
				index = -1
			}
			if index < 0 {
				break
			}
		}

		var incoming <-chan streamJob = q.incoming
		if len(pending) >= cap(q.incoming) {
			incoming = nil
		}
		select {
		case <-q.shutdown:
			return
		case job := <-incoming:
			pending = append(pending, job)
		case scope := <-q.releases:
			scope = strings.TrimSpace(scope)
			if active[scope] > 1 {
				active[scope]--
			} else {
				delete(active, scope)
			}
		}
	}
}

func nextEligibleJob(pending []streamJob, active map[string]int) int {
	for index, job := range pending {
		scope := strings.TrimSpace(job.ConcurrencyScope)
		if scope == "" || active[scope] < maxInt(job.ConcurrencyLimit, 1) {
			return index
		}
	}
	return -1
}
