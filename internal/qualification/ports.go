package qualification

import (
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/store"
	"sync"
	"time"
)

type Clock interface{ Now() time.Time }
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	repository          store.Repository
	engine              *protocol.Engine
	clock               Clock
	verificationMu      sync.Mutex
	verificationFlights map[string]*verificationFlight
}

type verificationFlight struct {
	done   chan struct{}
	result CertificateVerification
	err    error
}

func NewService(repository store.Repository, engine *protocol.Engine) *Service {
	return &Service{repository: repository, engine: engine, clock: systemClock{}}
}

func NewServiceWithClock(repository store.Repository, engine *protocol.Engine, clock Clock) *Service {
	return &Service{repository: repository, engine: engine, clock: clock}
}
