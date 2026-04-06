package selector

import (
	"fmt"
	"sort"

	"nodes_check/internal/precheck"
	iprobe "nodes_check/internal/probe"
)

const (
	CategoryAsia    = "\u4e9a\u6d32"
	CategoryEurope  = "\u6b27\u6d32"
	CategoryAmerica = "\u7f8e\u6d32"
	CategoryOther   = "\u5176\u4ed6"
	CategoryMobile  = "\u79fb\u52a8"
	CategoryUnicom  = "\u8054\u901a"
	CategoryTelecom = "\u7535\u4fe1"
)

type Candidate struct {
	Precheck  precheck.Result
	Aggregate iprobe.AggregateResult
}

type Selected struct {
	Precheck  precheck.Result
	Best      iprobe.SingleResult
	Aggregate iprobe.AggregateResult
	FinalName string
}

type PublishPlan struct {
	Counts map[string]int
}

func NewPublishPlan(counts map[string]int) PublishPlan {
	clean := make(map[string]int, len(counts))
	for k, v := range counts {
		clean[k] = v
	}
	return PublishPlan{Counts: clean}
}

func Select(candidates []Candidate, plan PublishPlan, prefix string) []Selected {
	grouped := make(map[string][]Selected)
	for _, candidate := range candidates {
		best := bestResult(candidate.Aggregate)
		if best == nil {
			continue
		}
		grouped[candidate.Precheck.Category] = append(grouped[candidate.Precheck.Category], Selected{
			Precheck:  candidate.Precheck,
			Best:      *best,
			Aggregate: candidate.Aggregate,
		})
	}

	selected := make([]Selected, 0)
	for category, items := range grouped {
		sort.Slice(items, func(i, j int) bool {
			if items[i].Aggregate.Summary.SuccessCount != items[j].Aggregate.Summary.SuccessCount {
				return items[i].Aggregate.Summary.SuccessCount > items[j].Aggregate.Summary.SuccessCount
			}
			if items[i].Aggregate.Summary.AvgDelayMS != items[j].Aggregate.Summary.AvgDelayMS {
				return items[i].Aggregate.Summary.AvgDelayMS < items[j].Aggregate.Summary.AvgDelayMS
			}
			return items[i].Best.RealDelayMS < items[j].Best.RealDelayMS
		})

		limit := plan.Counts[category]
		if limit <= 0 {
			continue
		}
		if len(items) > limit {
			items = items[:limit]
		}
		rename(items, category, prefix)
		selected = append(selected, items...)
	}

	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Precheck.Category != selected[j].Precheck.Category {
			return selected[i].Precheck.Category < selected[j].Precheck.Category
		}
		return selected[i].Best.RealDelayMS < selected[j].Best.RealDelayMS
	})
	return selected
}

func rename(items []Selected, category string, prefix string) {
	for i := range items {
		index := i + 1
		switch category {
		case CategoryMobile, CategoryUnicom, CategoryTelecom:
			items[i].FinalName = fmt.Sprintf("%s%s -%d", prefix, category, index)
		default:
			if items[i].Precheck.SubRegion != "" {
				items[i].FinalName = fmt.Sprintf("%s%s-%s-%d", prefix, category, items[i].Precheck.SubRegion, index)
			} else {
				items[i].FinalName = fmt.Sprintf("%s%s -%d", prefix, category, index)
			}
		}
	}
}

func bestResult(aggregate iprobe.AggregateResult) *iprobe.SingleResult {
	var best *iprobe.SingleResult
	for i := range aggregate.Results {
		item := aggregate.Results[i]
		if !item.Success {
			continue
		}
		if best == nil || item.RealDelayMS < best.RealDelayMS {
			copyItem := item
			best = &copyItem
		}
	}
	return best
}
