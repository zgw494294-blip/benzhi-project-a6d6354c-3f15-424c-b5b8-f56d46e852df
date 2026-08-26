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
	repository store.Repository
	engine     *protocol.Engine
	clock      Clock
	viewMu     sync.Mutex
	views      map[string]cachedAssessmentView
}

func NewService(repository store.Repository, engine *protocol.Engine) *Service {
	return &Service{repository: repository, engine: engine, clock: systemClock{}, views: map[string]cachedAssessmentView{}}
}

func NewServiceWithClock(repository store.Repository, engine *protocol.Engine, clock Clock) *Service {
	return &Service{repository: repository, engine: engine, clock: clock, views: map[string]cachedAssessmentView{}}
}
