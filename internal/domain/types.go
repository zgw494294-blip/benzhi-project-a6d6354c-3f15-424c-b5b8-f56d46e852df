package domain

import "time"

type Status string

const (
	StatusDraft          Status = "draft"
	StatusProtocolFrozen Status = "protocol_frozen"
	StatusObserving      Status = "observing"
	StatusPendingReview  Status = "pending_review"
	StatusReturned       Status = "returned"
	StatusApproved       Status = "approved"
	StatusSealed         Status = "sealed"
)

type Assessment struct {
	ID                   string    `json:"id"`
	LotCode              string    `json:"lotCode"`
	SpeciesName          string    `json:"speciesName"`
	HarvestYear          int       `json:"harvestYear"`
	SubmittedQuantity    int       `json:"submittedQuantity"`
	PretreatmentBoundary string    `json:"pretreatmentBoundary"`
	Status               Status    `json:"status"`
	Version              int64     `json:"version"`
	CreatedAt            time.Time `json:"createdAt"`
}

type ProtocolSnapshot struct {
	AssessmentID           string    `json:"assessmentId"`
	SnapshotNo             int       `json:"snapshotNo"`
	ReplicateCount         int       `json:"replicateCount"`
	SeedsPerReplicate      int       `json:"seedsPerReplicate"`
	TemperatureMinC        float64   `json:"temperatureMinC"`
	TemperatureMaxC        float64   `json:"temperatureMaxC"`
	ObservationDays        []int     `json:"observationDays"`
	TerminationDay         int       `json:"terminationDay"`
	MinimumGerminationRate float64   `json:"minimumGerminationRate"`
	MaximumDispersion      float64   `json:"maximumDispersion"`
	FrozenAt               time.Time `json:"frozenAt"`
	ContentDigest          string    `json:"contentDigest"`
}

type ReplicateKind string
type ReplicateStatus string

const (
	ReplicateOriginal ReplicateKind   = "original"
	ReplicateRetest   ReplicateKind   = "retest"
	ReplicateActive   ReplicateStatus = "active"
	ReplicateVoid     ReplicateStatus = "void"
	ReplicateComplete ReplicateStatus = "complete"
)

type Replicate struct {
	ID                string          `json:"id"`
	AssessmentID      string          `json:"assessmentId"`
	Label             string          `json:"label"`
	Kind              ReplicateKind   `json:"kind"`
	SourceReplicateID string          `json:"sourceReplicateId,omitempty"`
	SownQuantity      int             `json:"sownQuantity"`
	StartedAt         time.Time       `json:"startedAt"`
	Status            ReplicateStatus `json:"status"`
	VoidReason        string          `json:"voidReason,omitempty"`
}

type Observation struct {
	ID                string    `json:"id"`
	ReplicateID       string    `json:"replicateId"`
	DayNo             int       `json:"dayNo"`
	NormalGerminated  int       `json:"normalGerminated"`
	AbnormalSeedlings int       `json:"abnormalSeedlings"`
	HardSeeds         int       `json:"hardSeeds"`
	DeadSeeds         int       `json:"deadSeeds"`
	UngerminatedSeeds int       `json:"ungerminatedSeeds"`
	RevisionNo        int       `json:"revisionNo"`
	RecordedBy        string    `json:"recordedBy"`
	RecordedAt        time.Time `json:"recordedAt"`
}

type DeviationStatus string

const (
	DeviationOpen   DeviationStatus = "open"
	DeviationClosed DeviationStatus = "closed"
)

type Deviation struct {
	ID                   string            `json:"id"`
	AssessmentID         string            `json:"assessmentId"`
	Category             string            `json:"category"`
	OccurredAt           time.Time         `json:"occurredAt"`
	AffectedReplicateIDs []string          `json:"affectedReplicateIds"`
	Description          string            `json:"description"`
	Disposition          string            `json:"disposition"`
	RetestReplicateIDs   []string          `json:"retestReplicateIds"`
	RetestSeedQuantity   int               `json:"retestSeedQuantity"`
	Status               DeviationStatus   `json:"status"`
	ClosedAt             *time.Time        `json:"closedAt,omitempty"`
	ReadyToClose         bool              `json:"readyToClose,omitempty"`
	ReadinessIssues      map[string]string `json:"readinessIssues,omitempty"`
}

type SampleBoundary struct {
	SubmittedQuantity int `json:"submittedQuantity"`
	OriginalUsed      int `json:"originalUsed"`
	RetestUsed        int `json:"retestUsed"`
	Available         int `json:"available"`
}

type RuleHit struct {
	Code     string `json:"code"`
	Passed   bool   `json:"passed"`
	Blocking bool   `json:"blocking"`
	Message  string `json:"message"`
}

type Metrics struct {
	FinalGerminationRate float64   `json:"finalGerminationRate"`
	Dispersion           float64   `json:"dispersion"`
	ThresholdDay         int       `json:"thresholdDay"`
	Decision             string    `json:"decision"`
	RuleHits             []RuleHit `json:"ruleHits"`
	CalculatedAt         time.Time `json:"calculatedAt"`
	SourceVersion        int64     `json:"sourceVersion"`
}

type ReviewEvidenceArea string

const (
	EvidenceProtocol    ReviewEvidenceArea = "protocol"
	EvidenceObservation ReviewEvidenceArea = "observation"
	EvidenceDeviation   ReviewEvidenceArea = "deviation"
	EvidenceRules       ReviewEvidenceArea = "rules"
)

type ReviewItem struct {
	ID                  string             `json:"id"`
	EvidenceArea        ReviewEvidenceArea `json:"evidenceArea"`
	Problem             string             `json:"problem"`
	Requirement         string             `json:"requirement"`
	ReplicateIDs        []string           `json:"replicateIds,omitempty"`
	ReturnedVersion     int64              `json:"returnedVersion"`
	BlockingRuleCodes   []string           `json:"blockingRuleCodes,omitempty"`
	Resolved            bool               `json:"resolved"`
	CompletionStatement string             `json:"completionStatement,omitempty"`
	ResolvedVersion     int64              `json:"resolvedVersion,omitempty"`
	ResolvedAt          *time.Time         `json:"resolvedAt,omitempty"`
	CanResolve          bool               `json:"canResolve,omitempty"`
	ResolveBlocker      string             `json:"resolveBlocker,omitempty"`
}

type Review struct {
	ReviewedBy string       `json:"reviewedBy"`
	Reason     string       `json:"reason"`
	Approved   bool         `json:"approved"`
	ReviewedAt time.Time    `json:"reviewedAt"`
	Items      []ReviewItem `json:"items,omitempty"`
}

type QualificationCertificate struct {
	CertificateNo        string    `json:"certificateNo"`
	AssessmentID         string    `json:"assessmentId"`
	ProtocolDigest       string    `json:"protocolDigest"`
	InputDigest          string    `json:"inputDigest"`
	FinalGerminationRate float64   `json:"finalGerminationRate"`
	Dispersion           float64   `json:"dispersion"`
	ThresholdDay         int       `json:"thresholdDay"`
	Decision             string    `json:"decision"`
	ReviewedBy           string    `json:"reviewedBy"`
	ApprovedAt           time.Time `json:"approvedAt"`
	EventChainDigest     string    `json:"eventChainDigest"`
	CertificateDigest    string    `json:"certificateDigest"`
}
