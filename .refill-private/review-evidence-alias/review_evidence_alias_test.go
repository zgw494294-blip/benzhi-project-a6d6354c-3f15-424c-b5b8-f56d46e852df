package review_evidence_alias_test

import (
	"encoding/json"
	"seed-vigor-gate/internal/domain"
	"testing"
	"time"
)

func TestUnrelatedAuditStringCannotResolveScopedReviewItem(t *testing.T) {
	observationData, err := json.Marshal(domain.ObservationRecordedData{Observation: domain.Observation{
		ID: "obs-r2", ReplicateID: "r2", DayNo: 7, NormalGerminated: 9,
		DeadSeeds: 1, RevisionNo: 1, RecordedBy: "r1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	aggregate := domain.EmptyAggregate()
	aggregate.Assessment = domain.Assessment{ID: "review-a", Status: domain.StatusReturned, Version: 7}
	aggregate.Replicates["r1"] = domain.Replicate{ID: "r1", AssessmentID: "review-a"}
	aggregate.Replicates["r2"] = domain.Replicate{ID: "r2", AssessmentID: "review-a"}
	aggregate.ReviewItems["item-1"] = domain.ReviewItem{
		ID: "item-1", EvidenceArea: domain.EvidenceObservation,
		ReplicateIDs: []string{"r1"}, ReturnedVersion: 5,
	}
	aggregate.Metrics = &domain.Metrics{SourceVersion: 6}
	aggregate.Audit = []domain.AuditEntry{
		{Version: 6, Type: domain.EventObservationRecorded, Data: observationData},
		{Version: 7, Type: domain.EventMetricsCalculated},
	}
	_, err = aggregate.ResolveReviewItem("item-1", "已完成", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatalf("TestUnrelatedAuditStringCannotResolveScopedReviewItem: recordedBy text was mistaken for scoped replicate evidence")
	}
}
