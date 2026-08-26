package qualification

import (
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/store"
	"time"
)

type Clock interface{ Now() time.Time }
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	repository                store.Repository
	engine                    *protocol.Engine
	clock                     Clock
	verificationCertificateNo string
	verification              *CertificateVerification
}

func NewService(repository store.Repository, engine *protocol.Engine) *Service {
	return &Service{repository: repository, engine: engine, clock: systemClock{}}
}

func NewServiceWithClock(repository store.Repository, engine *protocol.Engine, clock Clock) *Service {
	return &Service{repository: repository, engine: engine, clock: clock}
}
