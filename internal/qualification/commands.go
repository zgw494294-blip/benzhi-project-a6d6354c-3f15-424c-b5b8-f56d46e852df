package qualification

import (
	"seed-vigor-gate/internal/domain"
	"time"
)

type CreateCommand struct {
	ID                   string `json:"id"`
	LotCode              string `json:"lotCode"`
	SpeciesName          string `json:"speciesName"`
	HarvestYear          int    `json:"harvestYear"`
	SubmittedQuantity    int    `json:"submittedQuantity"`
	PretreatmentBoundary string `json:"pretreatmentBoundary"`
}

type Versioned struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

type FreezeProtocolCommand struct {
	Versioned
	ReplicateCount         int     `json:"replicateCount"`
	SeedsPerReplicate      int     `json:"seedsPerReplicate"`
	TemperatureMinC        float64 `json:"temperatureMinC"`
	TemperatureMaxC        float64 `json:"temperatureMaxC"`
	ObservationDays        []int   `json:"observationDays"`
	TerminationDay         int     `json:"terminationDay"`
	MinimumGerminationRate float64 `json:"minimumGerminationRate"`
	MaximumDispersion      float64 `json:"maximumDispersion"`
}

type ReplicateInput struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	SownQuantity int       `json:"sownQuantity"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
}

type PlaceReplicatesCommand struct {
	Versioned
	Replicates []ReplicateInput `json:"replicates"`
}
type StartCommand struct{ Versioned }

type RecordObservationCommand struct {
	Versioned
	ID                string `json:"id"`
	ReplicateID       string `json:"replicateId"`
	DayNo             int    `json:"dayNo"`
	NormalGerminated  int    `json:"normalGerminated"`
	AbnormalSeedlings int    `json:"abnormalSeedlings"`
	HardSeeds         int    `json:"hardSeeds"`
	DeadSeeds         int    `json:"deadSeeds"`
	UngerminatedSeeds int    `json:"ungerminatedSeeds"`
	RecordedBy        string `json:"recordedBy"`
}

type BatchObservationInput struct {
	ID                string `json:"id"`
	ReplicateID       string `json:"replicateId"`
	NormalGerminated  *int   `json:"normalGerminated"`
	AbnormalSeedlings *int   `json:"abnormalSeedlings"`
	HardSeeds         *int   `json:"hardSeeds"`
	DeadSeeds         *int   `json:"deadSeeds"`
	UngerminatedSeeds *int   `json:"ungerminatedSeeds"`
}

type RecordObservationBatchCommand struct {
	Versioned
	DayNo        int                     `json:"dayNo"`
	RecordedBy   string                  `json:"recordedBy"`
	Observations []BatchObservationInput `json:"observations"`
	ValidateOnly bool                    `json:"validateOnly,omitempty"`
}

type RegisterDeviationCommand struct {
	Versioned
	ID                   string    `json:"id"`
	Category             string    `json:"category"`
	OccurredAt           time.Time `json:"occurredAt,omitempty"`
	AffectedReplicateIDs []string  `json:"affectedReplicateIds"`
	Description          string    `json:"description"`
	Disposition          string    `json:"disposition"`
}

type CloseDeviationCommand struct {
	Versioned
	DeviationID string `json:"deviationId"`
}
type CalculateCommand struct{ Versioned }
type ReviewCommand struct {
	Versioned
	Reviewer string `json:"reviewer"`
	Reason   string `json:"reason"`
}
type ReviewItemCommand struct {
	ID           string   `json:"id,omitempty"`
	EvidenceArea string   `json:"evidenceArea"`
	Problem      string   `json:"problem"`
	Requirement  string   `json:"requirement"`
	ReplicateIDs []string `json:"replicateIds,omitempty"`
}
type ReturnReviewCommand struct {
	Versioned
	Reviewer string              `json:"reviewer"`
	Reason   string              `json:"reason"`
	Items    []ReviewItemCommand `json:"items"`
}
type ResolveReviewItemCommand struct {
	Versioned
	ItemID              string `json:"itemId"`
	CompletionStatement string `json:"completionStatement"`
}
type ResubmitReviewCommand struct {
	Versioned
	SubmittedBy string `json:"submittedBy"`
}
type SealCommand struct{ Versioned }

func (c FreezeProtocolCommand) Snapshot() domain.ProtocolSnapshot {
	return domain.ProtocolSnapshot{ReplicateCount: c.ReplicateCount, SeedsPerReplicate: c.SeedsPerReplicate, TemperatureMinC: c.TemperatureMinC, TemperatureMaxC: c.TemperatureMaxC, ObservationDays: append([]int(nil), c.ObservationDays...), TerminationDay: c.TerminationDay, MinimumGerminationRate: c.MinimumGerminationRate, MaximumDispersion: c.MaximumDispersion}
}
