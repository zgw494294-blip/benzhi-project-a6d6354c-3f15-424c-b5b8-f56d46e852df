package protocol

import (
	"fmt"
	"seed-vigor-gate/internal/domain"
	"sort"
)

type Issue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Validate(snapshot domain.ProtocolSnapshot, submittedQuantity int) []Issue {
	issues := make([]Issue, 0)
	if snapshot.ReplicateCount < 2 || snapshot.ReplicateCount > 20 {
		issues = append(issues, Issue{"replicateCount", "RANGE", "重复组数量必须在 2 到 20 之间"})
	}
	if snapshot.SeedsPerReplicate <= 0 || snapshot.SeedsPerReplicate > 1000 {
		issues = append(issues, Issue{"seedsPerReplicate", "RANGE", "每组粒数必须在 1 到 1000 之间"})
	}
	if snapshot.ReplicateCount*snapshot.SeedsPerReplicate > submittedQuantity {
		issues = append(issues, Issue{"seedsPerReplicate", "SAMPLE_BOUNDARY", fmt.Sprintf("计划用种量不能超过送检数量 %d", submittedQuantity)})
	}
	if snapshot.TemperatureMinC < 0 || snapshot.TemperatureMaxC > 50 || snapshot.TemperatureMinC >= snapshot.TemperatureMaxC {
		issues = append(issues, Issue{"temperatureRange", "INVALID_RANGE", "温度区间须位于 0–50°C 且下限小于上限"})
	}
	if len(snapshot.ObservationDays) == 0 {
		issues = append(issues, Issue{"observationDays", "REQUIRED", "至少设置一个观察日"})
	} else {
		seen := map[int]bool{}
		last := 0
		for _, day := range snapshot.ObservationDays {
			if day <= 0 || seen[day] || day <= last {
				issues = append(issues, Issue{"observationDays", "ORDER", "观察日必须为严格递增的正整数且不能重复"})
				break
			}
			seen[day], last = true, day
		}
		if !seen[snapshot.TerminationDay] {
			issues = append(issues, Issue{"terminationDay", "NOT_SCHEDULED", "终止日必须包含在观察日程中"})
		}
		if snapshot.TerminationDay != snapshot.ObservationDays[len(snapshot.ObservationDays)-1] {
			issues = append(issues, Issue{"terminationDay", "NOT_LAST", "终止日必须是日程中的最后一天"})
		}
	}
	if snapshot.MinimumGerminationRate <= 0 || snapshot.MinimumGerminationRate > 100 {
		issues = append(issues, Issue{"minimumGerminationRate", "RANGE", "最低发芽率必须大于 0 且不超过 100"})
	}
	if snapshot.MaximumDispersion < 0 || snapshot.MaximumDispersion > 100 {
		issues = append(issues, Issue{"maximumDispersion", "RANGE", "最大离散度必须位于 0 到 100 之间"})
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Field == issues[j].Field {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Field < issues[j].Field
	})
	return issues
}
