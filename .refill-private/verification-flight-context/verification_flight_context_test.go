package verificationflightcontext_test

import (
	"context"
	"errors"
	"seed-vigor-gate/internal/domain"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"sync"
	"testing"
)

type controlledRepository struct {
	loadStarted chan struct{}
	loadAllowed chan struct{}
	startOnce   sync.Once
}

func (r *controlledRepository) LoadCertificateMaterial(ctx context.Context, _ string) (store.CertificateMaterial, error) {
	r.startOnce.Do(func() { close(r.loadStarted) })
	select {
	case <-ctx.Done():
		return store.CertificateMaterial{}, ctx.Err()
	case <-r.loadAllowed:
		return store.CertificateMaterial{}, nil
	}
}

func (r *controlledRepository) Load(context.Context, string) (*domain.Aggregate, error) {
	return nil, errors.New("unexpected Load")
}

func (r *controlledRepository) Append(context.Context, string, int64, string, domain.Event, string) (store.Receipt, error) {
	return store.Receipt{}, errors.New("unexpected Append")
}

func (r *controlledRepository) LookupReceipt(context.Context, string) (store.Receipt, bool) {
	return store.Receipt{}, false
}

func (r *controlledRepository) ChainDigest(context.Context, string) (string, error) {
	return "", errors.New("unexpected ChainDigest")
}

func (r *controlledRepository) FindCertificate(context.Context, string) (domain.QualificationCertificate, error) {
	return domain.QualificationCertificate{}, errors.New("unexpected FindCertificate")
}

type observedContext struct {
	context.Context
	doneObserved chan struct{}
	once         sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.doneObserved) })
	return c.Context.Done()
}

func TestHealthyWaiterDoesNotInheritLeaderCancellation(t *testing.T) {
	repository := &controlledRepository{loadStarted: make(chan struct{}), loadAllowed: make(chan struct{})}
	service := qualification.NewService(repository, protocol.NewEngine())
	leaderContext, cancelLeader := context.WithCancel(context.Background())
	leaderResult := make(chan error, 1)
	followerResult := make(chan error, 1)

	go func() {
		_, err := service.VerifyCertificate(leaderContext, "SVG-SHARED-0001")
		leaderResult <- err
	}()
	<-repository.loadStarted

	followerContext := &observedContext{Context: context.Background(), doneObserved: make(chan struct{})}
	go func() {
		_, err := service.VerifyCertificate(followerContext, "SVG-SHARED-0001")
		followerResult <- err
	}()
	<-followerContext.doneObserved

	cancelLeader()
	if err := <-leaderResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader should observe its own cancellation, got %v", err)
	}
	close(repository.loadAllowed)
	if err := <-followerResult; err != nil {
		t.Fatalf("healthy follower inherited leader cancellation: %v", err)
	}
}
