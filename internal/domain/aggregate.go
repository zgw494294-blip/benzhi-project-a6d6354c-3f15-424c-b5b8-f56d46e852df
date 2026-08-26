package domain

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Aggregate struct {
	Assessment   Assessment                `json:"assessment"`
	Protocol     *ProtocolSnapshot         `json:"protocol,omitempty"`
	Replicates   map[string]Replicate      `json:"replicates"`
	Observations map[string][]Observation  `json:"observations"`
	Deviations   map[string]Deviation      `json:"deviations"`
	Metrics      *Metrics                  `json:"metrics,omitempty"`
	Reviews      []Review                  `json:"reviews"`
	ReviewItems  map[string]ReviewItem     `json:"reviewItems"`
	Certificate  *QualificationCertificate `json:"certificate,omitempty"`
	Audit        []AuditEntry              `json:"audit"`
}

type AuditEntry struct {
	Version    int64           `json:"version"`
	Type       string          `json:"type"`
	OccurredAt string          `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}

func EmptyAggregate() *Aggregate {
	return &Aggregate{Replicates: map[string]Replicate{}, Observations: map[string][]Observation{}, Deviations: map[string]Deviation{}, ReviewItems: map[string]ReviewItem{}}
}

func Rehydrate(events []Event) (*Aggregate, error) {
	a := EmptyAggregate()
	for _, event := range events {
		if err := a.Apply(event); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (a *Aggregate) Apply(event Event) error {
	switch event.Type {
	case EventAssessmentCreated:
		if err := json.Unmarshal(event.Data, &a.Assessment); err != nil {
			return err
		}
	case EventProtocolFrozen:
		var value ProtocolSnapshot
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.Protocol = &value
		a.Assessment.Status = StatusProtocolFrozen
	case EventReplicatesPlaced:
		var value ReplicatesPlacedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		for _, replicate := range value.Replicates {
			a.Replicates[replicate.ID] = replicate
		}
	case EventObservationStarted:
		a.Assessment.Status = StatusObserving
	case EventObservationRecorded:
		var value ObservationRecordedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.applyObservation(value.Observation)
		a.Metrics = nil
	case EventObservationBatchRecorded:
		var value ObservationBatchRecordedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		for _, observation := range value.Observations {
			a.applyObservation(observation)
		}
		a.Metrics = nil
	case EventDeviationRegistered:
		wasReturned := a.Assessment.Status == StatusReturned
		var value DeviationRegisteredData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.Deviations[value.Deviation.ID] = value.Deviation
		for id, reason := range value.Voided {
			r := a.Replicates[id]
			r.Status = ReplicateVoid
			r.VoidReason = reason
			a.Replicates[id] = r
		}
		for _, r := range value.Retests {
			a.Replicates[r.ID] = r
		}
		a.Metrics = nil
		if wasReturned {
			a.Assessment.Status = StatusReturned
		} else {
			a.Assessment.Status = StatusObserving
		}
	case EventDeviationClosed:
		wasReturned := a.Assessment.Status == StatusReturned
		var value DeviationClosedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		d := a.Deviations[value.ID]
		d.Status = DeviationClosed
		d.ClosedAt = &value.ClosedAt
		a.Deviations[value.ID] = d
		a.Metrics = nil
		if wasReturned {
			a.Assessment.Status = StatusReturned
		} else {
			a.Assessment.Status = StatusObserving
		}
	case EventMetricsCalculated:
		var value MetricsCalculatedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.Metrics = &value.Metrics
		if a.Assessment.Status != StatusReturned {
			a.Assessment.Status = StatusPendingReview
		}
	case EventReviewReturned:
		var value ReviewData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.Reviews = append(a.Reviews, value.Review)
		for _, item := range value.Review.Items {
			a.ReviewItems[item.ID] = item
		}
		a.Assessment.Status = StatusReturned
	case EventReviewItemResolved:
		var value ReviewItemResolvedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		item := a.ReviewItems[value.ItemID]
		item.Resolved = true
		item.CompletionStatement = value.CompletionStatement
		item.ResolvedVersion = value.EvidenceVersion
		resolvedAt := value.ResolvedAt
		item.ResolvedAt = &resolvedAt
		a.ReviewItems[value.ItemID] = item
	case EventReviewResubmitted:
		a.Assessment.Status = StatusPendingReview
	case EventReviewApproved:
		var value ReviewData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.Reviews = append(a.Reviews, value.Review)
		a.Assessment.Status = StatusApproved
	case EventCertificateSealed:
		var value CertificateSealedData
		if err := json.Unmarshal(event.Data, &value); err != nil {
			return err
		}
		a.Certificate = &value.Certificate
		a.Assessment.Status = StatusSealed
	default:
		return fmt.Errorf("未知领域事件 %q", event.Type)
	}
	a.Assessment.Version++
	a.Audit = append(a.Audit, AuditEntry{Version: a.Assessment.Version, Type: event.Type, OccurredAt: event.OccurredAt.Format("2006-01-02T15:04:05Z07:00"), Data: append(json.RawMessage(nil), event.Data...)})
	return nil
}

func (a *Aggregate) applyObservation(observation Observation) {
	list := a.Observations[observation.ReplicateID]
	a.Observations[observation.ReplicateID] = append(list, observation)
	if a.Protocol != nil && observation.DayNo == a.Protocol.TerminationDay {
		replicate := a.Replicates[observation.ReplicateID]
		if replicate.Status != ReplicateVoid {
			replicate.Status = ReplicateComplete
			a.Replicates[observation.ReplicateID] = replicate
		}
	}
}

func (a *Aggregate) SortedReplicates() []Replicate {
	values := make([]Replicate, 0, len(a.Replicates))
	for _, value := range a.Replicates {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Label < values[j].Label })
	return values
}

func (a *Aggregate) SortedDeviations() []Deviation {
	values := make([]Deviation, 0, len(a.Deviations))
	for _, value := range a.Deviations {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].OccurredAt.Before(values[j].OccurredAt) })
	return values
}

func (a *Aggregate) SortedReviewItems() []ReviewItem {
	values := make([]ReviewItem, 0, len(a.ReviewItems))
	for _, value := range a.ReviewItems {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].ReturnedVersion == values[j].ReturnedVersion {
			return values[i].ID < values[j].ID
		}
		return values[i].ReturnedVersion < values[j].ReturnedVersion
	})
	return values
}

func (a *Aggregate) MetricsCurrent() bool {
	if a.Metrics == nil || a.Metrics.SourceVersion <= 0 {
		return false
	}
	calculatedVersion := a.Metrics.SourceVersion + 1
	found := false
	for _, entry := range a.Audit {
		if entry.Version == calculatedVersion && entry.Type == EventMetricsCalculated {
			found = true
		}
		if entry.Version > calculatedVersion && (entry.Type == EventObservationRecorded || entry.Type == EventObservationBatchRecorded || entry.Type == EventDeviationRegistered || entry.Type == EventDeviationClosed) {
			return false
		}
	}
	return found
}
