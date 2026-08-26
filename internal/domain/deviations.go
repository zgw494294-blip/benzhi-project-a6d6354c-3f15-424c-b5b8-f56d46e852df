package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type RegisterDeviationInput struct {
	ID                   string
	Category             string
	OccurredAt           time.Time
	AffectedReplicateIDs []string
	Description          string
	Disposition          string
}

func (a *Aggregate) RegisterDeviation(input RegisterDeviationInput, at time.Time) (Event, error) {
	if a.Assessment.Status != StatusObserving && a.Assessment.Status != StatusReturned && a.Assessment.Status != StatusPendingReview {
		return Event{}, State("observing、returned 或 pending_review", a.Assessment.Status)
	}
	if _, exists := a.Deviations[input.ID]; exists {
		return Event{}, Invalid("异常编号重复", nil)
	}
	allowed := map[string]bool{"contamination": true, "temperature_interruption": true, "count_invalid": true}
	if !allowed[input.Category] {
		return Event{}, Invalid("异常类别仅支持 contamination、temperature_interruption 或 count_invalid", nil)
	}
	if len(input.AffectedReplicateIDs) == 0 {
		return Event{}, Invalid("至少选择一个受影响重复组", nil)
	}
	if strings.TrimSpace(input.Description) == "" || strings.TrimSpace(input.Disposition) == "" {
		return Event{}, Invalid("异常描述和处置决定不能为空", nil)
	}
	voided := map[string]string{}
	retests := make([]Replicate, 0, len(input.AffectedReplicateIDs))
	retestIDs := make([]string, 0, len(input.AffectedReplicateIDs))
	seen := map[string]bool{}
	demand := 0
	for _, id := range input.AffectedReplicateIDs {
		if seen[id] {
			return Event{}, Invalid("受影响重复组不能重复", nil)
		}
		seen[id] = true
		r, ok := a.Replicates[id]
		if !ok || r.AssessmentID != a.Assessment.ID || r.Status == ReplicateVoid {
			return Event{}, Invalid("受影响重复组不存在、已作废或不属于本评定", map[string]string{"replicates." + id: "不是可用重复组"})
		}
		for _, deviation := range a.Deviations {
			if deviation.Status != DeviationOpen {
				continue
			}
			occupiedGroups := append(append([]string(nil), deviation.AffectedReplicateIDs...), deviation.RetestReplicateIDs...)
			for _, occupied := range occupiedGroups {
				if occupied == id {
					return Event{}, Invalid("重复组已被未闭环异常占用", map[string]string{"replicates." + id: "关联异常 " + deviation.ID})
				}
			}
		}
		demand += r.SownQuantity
		voided[id] = input.Category + ": " + input.Description
		retestID := input.ID + "-retest-" + id
		retests = append(retests, Replicate{ID: retestID, AssessmentID: a.Assessment.ID, Label: "RT-" + r.Label, Kind: ReplicateRetest, SourceReplicateID: id, SownQuantity: r.SownQuantity, StartedAt: at.UTC(), Status: ReplicateActive})
		retestIDs = append(retestIDs, retestID)
	}
	boundary := a.SampleBoundary()
	if demand > boundary.Available {
		groups := append([]string(nil), input.AffectedReplicateIDs...)
		sort.Strings(groups)
		return Event{}, Invalid("剩余样本不足，不能创建全部定向复测组", map[string]string{
			"requiredQuantity":  fmt.Sprintf("%d", demand),
			"availableQuantity": fmt.Sprintf("%d", boundary.Available),
			"replicateIds":      strings.Join(groups, ","),
		})
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = at
	}
	value := Deviation{ID: input.ID, AssessmentID: a.Assessment.ID, Category: input.Category, OccurredAt: input.OccurredAt.UTC(), AffectedReplicateIDs: append([]string(nil), input.AffectedReplicateIDs...), Description: strings.TrimSpace(input.Description), Disposition: strings.TrimSpace(input.Disposition), RetestReplicateIDs: retestIDs, RetestSeedQuantity: demand, Status: DeviationOpen}
	return NewEvent(EventDeviationRegistered, at, DeviationRegisteredData{Deviation: value, Voided: voided, Retests: retests})
}

func (a *Aggregate) CloseDeviation(id string, at time.Time) (Event, error) {
	d, ok := a.Deviations[id]
	if !ok {
		return Event{}, &DomainError{Code: CodeNotFound, Message: "异常不存在"}
	}
	if d.Status == DeviationClosed {
		return Event{}, Invalid("异常已闭环", nil)
	}
	fields := a.DeviationReadiness(d)
	if len(fields) > 0 {
		return Event{}, Invalid("异常闭环就绪核验未通过", fields)
	}
	return NewEvent(EventDeviationClosed, at, DeviationClosedData{ID: id, ClosedAt: at.UTC()})
}

func (a *Aggregate) DeviationReadiness(d Deviation) map[string]string {
	fields := map[string]string{}
	if a.Protocol == nil {
		fields["protocol"] = "缺少冻结方案"
		return fields
	}
	for _, replicateID := range d.RetestReplicateIDs {
		latest := LatestObservations(a.Observations[replicateID])
		missing := make([]string, 0)
		for _, day := range a.Protocol.ObservationDays {
			observation, ok := latest[day]
			if !ok {
				missing = append(missing, fmt.Sprintf("D%d", day))
				continue
			}
			total := observation.NormalGerminated + observation.AbnormalSeedlings + observation.HardSeeds + observation.DeadSeeds + observation.UngerminatedSeeds
			if total != a.Replicates[replicateID].SownQuantity {
				missing = append(missing, fmt.Sprintf("D%d计数不守恒", day))
			}
		}
		if len(missing) > 0 {
			fields["retests."+replicateID+".observations"] = strings.Join(missing, ", ")
		}
	}
	if a.Metrics == nil || !a.MetricsCurrent() {
		fields["metrics"] = "资格指标尚未基于当前观测版本重新计算"
	}
	return fields
}

func (a *Aggregate) HasOpenDeviation() bool {
	for _, deviation := range a.Deviations {
		if deviation.Status == DeviationOpen {
			return true
		}
	}
	return false
}

func (a *Aggregate) SampleBoundary() SampleBoundary {
	boundary := SampleBoundary{SubmittedQuantity: a.Assessment.SubmittedQuantity}
	for _, replicate := range a.Replicates {
		if replicate.Kind == ReplicateOriginal {
			boundary.OriginalUsed += replicate.SownQuantity
		} else if replicate.Kind == ReplicateRetest {
			boundary.RetestUsed += replicate.SownQuantity
		}
	}
	boundary.Available = boundary.SubmittedQuantity - boundary.OriginalUsed - boundary.RetestUsed
	if boundary.Available < 0 {
		boundary.Available = 0
	}
	return boundary
}
