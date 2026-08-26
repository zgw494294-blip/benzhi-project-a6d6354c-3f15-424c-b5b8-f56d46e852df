package domain

import (
	"strings"
	"time"
)

type CreateAssessmentInput struct {
	ID                   string
	LotCode              string
	SpeciesName          string
	HarvestYear          int
	SubmittedQuantity    int
	PretreatmentBoundary string
}

func CreateAssessment(input CreateAssessmentInput, at time.Time) (Event, error) {
	fields := map[string]string{}
	if strings.TrimSpace(input.ID) == "" {
		fields["id"] = "不能为空"
	}
	if strings.TrimSpace(input.LotCode) == "" {
		fields["lotCode"] = "不能为空"
	}
	if strings.TrimSpace(input.SpeciesName) == "" {
		fields["speciesName"] = "不能为空"
	}
	if input.HarvestYear < 1900 || input.HarvestYear > at.Year() {
		fields["harvestYear"] = "必须是有效年份"
	}
	if input.SubmittedQuantity <= 0 {
		fields["submittedQuantity"] = "必须大于 0"
	}
	if strings.TrimSpace(input.PretreatmentBoundary) == "" {
		fields["pretreatmentBoundary"] = "不能为空"
	}
	if len(fields) > 0 {
		return Event{}, Invalid("批次登记不完整", fields)
	}
	value := Assessment{ID: input.ID, LotCode: strings.TrimSpace(input.LotCode), SpeciesName: strings.TrimSpace(input.SpeciesName), HarvestYear: input.HarvestYear, SubmittedQuantity: input.SubmittedQuantity, PretreatmentBoundary: strings.TrimSpace(input.PretreatmentBoundary), Status: StatusDraft, CreatedAt: at.UTC()}
	return NewEvent(EventAssessmentCreated, at, value)
}

func (a *Aggregate) FreezeProtocol(snapshot ProtocolSnapshot, at time.Time) (Event, error) {
	if a.Assessment.Status != StatusDraft {
		return Event{}, State("draft", a.Assessment.Status)
	}
	if a.Protocol != nil {
		return Event{}, State("尚未冻结方案", a.Assessment.Status)
	}
	snapshot.AssessmentID = a.Assessment.ID
	snapshot.SnapshotNo = 1
	snapshot.FrozenAt = at.UTC()
	return NewEvent(EventProtocolFrozen, at, snapshot)
}

func (a *Aggregate) PlaceReplicates(values []Replicate, at time.Time) (Event, error) {
	if a.Assessment.Status != StatusProtocolFrozen {
		return Event{}, State("protocol_frozen", a.Assessment.Status)
	}
	if len(a.Replicates) > 0 {
		return Event{}, Invalid("重复组已布置，不能覆盖", nil)
	}
	if a.Protocol == nil {
		return Event{}, Invalid("缺少冻结方案", nil)
	}
	if len(values) != a.Protocol.ReplicateCount {
		return Event{}, Invalid("重复组数量必须与冻结方案一致", map[string]string{"replicates": "数量不匹配"})
	}
	seen := map[string]bool{}
	total := 0
	for i := range values {
		r := &values[i]
		if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.Label) == "" {
			return Event{}, Invalid("重复组标识和编号不能为空", nil)
		}
		if seen[r.ID] || seen[r.Label] {
			return Event{}, Invalid("重复组标识或编号重复", nil)
		}
		seen[r.ID], seen[r.Label] = true, true
		if r.SownQuantity != a.Protocol.SeedsPerReplicate {
			return Event{}, Invalid("播种数量必须与冻结方案一致", map[string]string{"sownQuantity": "数量不匹配"})
		}
		if r.StartedAt.IsZero() {
			r.StartedAt = at.UTC()
		}
		r.AssessmentID = a.Assessment.ID
		r.Kind = ReplicateOriginal
		r.Status = ReplicateActive
		total += r.SownQuantity
	}
	if total > a.Assessment.SubmittedQuantity {
		return Event{}, Invalid("重复组用种量超出送检数量", nil)
	}
	return NewEvent(EventReplicatesPlaced, at, ReplicatesPlacedData{Replicates: values})
}

func (a *Aggregate) StartObservation(at time.Time) (Event, error) {
	if a.Assessment.Status != StatusProtocolFrozen {
		return Event{}, State("protocol_frozen", a.Assessment.Status)
	}
	if a.Protocol == nil || len(a.Replicates) != a.Protocol.ReplicateCount {
		return Event{}, Invalid("重复组缺失，不能开始观测", nil)
	}
	for _, r := range a.Replicates {
		if r.Status != ReplicateActive || r.SownQuantity != a.Protocol.SeedsPerReplicate {
			return Event{}, Invalid("重复组未正确布置", nil)
		}
	}
	return NewEvent(EventObservationStarted, at, ObservationStartedData{StartedAt: at.UTC()})
}
