package domain

import (
	"encoding/json"
	"time"
)

const (
	EventAssessmentCreated        = "assessment.created"
	EventProtocolFrozen           = "protocol.frozen"
	EventReplicatesPlaced         = "replicates.placed"
	EventObservationStarted       = "observation.started"
	EventObservationRecorded      = "observation.recorded"
	EventObservationBatchRecorded = "observation.batch_recorded"
	EventDeviationRegistered      = "deviation.registered"
	EventDeviationClosed          = "deviation.closed"
	EventMetricsCalculated        = "metrics.calculated"
	EventReviewReturned           = "review.returned"
	EventReviewItemResolved       = "review.item_resolved"
	EventReviewResubmitted        = "review.resubmitted"
	EventReviewApproved           = "review.approved"
	EventCertificateSealed        = "certificate.sealed"
)

type Event struct {
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}

func NewEvent(kind string, at time.Time, value any) (Event, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return Event{}, err
	}
	return Event{Type: kind, OccurredAt: at.UTC(), Data: b}, nil
}

type ReplicatesPlacedData struct {
	Replicates []Replicate `json:"replicates"`
}
type ObservationStartedData struct {
	StartedAt time.Time `json:"startedAt"`
}
type ObservationRecordedData struct {
	Observation Observation `json:"observation"`
}
type ObservationBatchRecordedData struct {
	DayNo        int           `json:"dayNo"`
	RecordedBy   string        `json:"recordedBy"`
	Observations []Observation `json:"observations"`
}
type DeviationRegisteredData struct {
	Deviation Deviation         `json:"deviation"`
	Voided    map[string]string `json:"voided"`
	Retests   []Replicate       `json:"retests"`
}
type DeviationClosedData struct {
	ID       string    `json:"id"`
	ClosedAt time.Time `json:"closedAt"`
}
type MetricsCalculatedData struct {
	Metrics Metrics `json:"metrics"`
}
type ReviewData struct {
	Review Review `json:"review"`
}
type ReviewItemResolvedData struct {
	ItemID              string    `json:"itemId"`
	CompletionStatement string    `json:"completionStatement"`
	EvidenceVersion     int64     `json:"evidenceVersion"`
	ResolvedAt          time.Time `json:"resolvedAt"`
}
type ReviewResubmittedData struct {
	SubmittedBy string    `json:"submittedBy"`
	SubmittedAt time.Time `json:"submittedAt"`
}
type CertificateSealedData struct {
	Certificate QualificationCertificate `json:"certificate"`
}
