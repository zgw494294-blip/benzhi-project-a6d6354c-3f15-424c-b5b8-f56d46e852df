package store

import (
	"context"
	"seed-vigor-gate/internal/domain"
)

const SchemaVersion = 1

type Receipt struct {
	IdempotencyKey   string `json:"idempotencyKey"`
	AssessmentID     string `json:"assessmentId"`
	Version          int64  `json:"version"`
	EventType        string `json:"eventType"`
	PayloadDigest    string `json:"payloadDigest,omitempty"`
	CertificateNo    string `json:"certificateNo,omitempty"`
	Replayed         bool   `json:"replayed,omitempty"`
	SuccessfulGroups int    `json:"successfulGroups"`
	TerminalReady    bool   `json:"terminalReady"`
}

type CertificateMaterial struct {
	Certificate        domain.QualificationCertificate
	PreSealAggregate   *domain.Aggregate
	SealEvent          domain.Event
	PreSealChainDigest string
	ChainValid         bool
}

type Repository interface {
	Load(context.Context, string) (*domain.Aggregate, error)
	Append(context.Context, string, int64, string, domain.Event, string) (Receipt, error)
	AppendCommand(context.Context, string, int64, string, string, domain.Event, string) (Receipt, error)
	LookupReceipt(context.Context, string) (Receipt, bool)
	ChainDigest(context.Context, string) (string, error)
	FindCertificate(context.Context, string) (domain.QualificationCertificate, error)
}

type CertificateMaterialRepository interface {
	LoadCertificateMaterial(context.Context, string) (CertificateMaterial, error)
}

type LedgerRecord struct {
	SchemaVersion  int          `json:"schemaVersion"`
	AssessmentID   string       `json:"assessmentId"`
	Sequence       int64        `json:"sequence"`
	PreviousDigest string       `json:"previousDigest"`
	Digest         string       `json:"digest"`
	IdempotencyKey string       `json:"idempotencyKey"`
	PayloadDigest  string       `json:"payloadDigest,omitempty"`
	CertificateNo  string       `json:"certificateNo,omitempty"`
	Event          domain.Event `json:"event"`
}

type snapshotFile struct {
	SchemaVersion int               `json:"schemaVersion"`
	LedgerSize    int64             `json:"ledgerSize"`
	Chains        map[string]string `json:"chains"`
	Versions      map[string]int64  `json:"versions"`
	Certificates  map[string]string `json:"certificates"`
}
