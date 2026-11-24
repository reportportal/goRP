package rpagent

import (
	"sort"
	"strings"
	"time"

	"github.com/reportportal/goRP/v5/pkg/gorp"
)

// ContainerResult is the aggregated outcome of one Describe/Context item.
type ContainerResult struct {
	ID     string
	Depth  int // 1 for a container directly under a suite
	Status gorp.Status
	End    time.Time // when the last spec beneath it ended; zero if no spec reported a time
}

// AggregateContainers derives the final status and end time of every container from the
// results of the specs beneath it. Containers are shared by all specs (and parallel
// workers) under them, so only the wrapper, after every worker has finished, can do this.
//
// A container is FAILED if any spec beneath it failed, was interrupted or timed out;
// SKIPPED if every spec beneath it was skipped or pending; PASSED otherwise.
//
// suiteIDs are the suite item IDs: when creating a container failed, a spec's chain
// falls back to its parent's ID, and those IDs (and consecutive repeats) are ignored.
//
// The result is ordered deepest first, the order in which ReportPortal requires items to
// be finished (an item cannot finish before its children).
func AggregateContainers(specs []SharedSpecStatus, suiteIDs map[string]bool) []ContainerResult {
	type tally struct {
		depth   int
		failed  bool
		skipped int
		total   int
		end     time.Time
	}
	tallies := make(map[string]*tally)

	for _, spec := range specs {
		state := strings.ToLower(spec.Status)
		prev := ""
		depth := 0
		for _, id := range spec.ContainerIDs {
			if id == "" || id == prev || suiteIDs[id] {
				continue
			}
			prev = id
			depth++

			t, ok := tallies[id]
			if !ok {
				t = &tally{depth: depth}
				tallies[id] = t
			}
			t.total++
			switch {
			case isFailureState(state):
				t.failed = true
			case state == "skipped" || state == "pending":
				t.skipped++
			}
			if spec.EndTime.After(t.end) {
				t.end = spec.EndTime
			}
		}
	}

	results := make([]ContainerResult, 0, len(tallies))
	for id, t := range tallies {
		status := gorp.Statuses.Passed
		switch {
		case t.failed:
			status = gorp.Statuses.Failed
		case t.total > 0 && t.skipped == t.total:
			status = gorp.Statuses.Skipped
		}
		results = append(
			results,
			ContainerResult{ID: id, Depth: t.depth, Status: status, End: t.end},
		)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Depth != results[j].Depth {
			return results[i].Depth > results[j].Depth
		}
		return results[i].ID < results[j].ID
	})
	return results
}

// LatestEnd returns the latest spec end time among specs, or the zero time if none has one.
func LatestEnd(specs []SharedSpecStatus) time.Time {
	var latest time.Time
	for _, s := range specs {
		if s.EndTime.After(latest) {
			latest = s.EndTime
		}
	}
	return latest
}
