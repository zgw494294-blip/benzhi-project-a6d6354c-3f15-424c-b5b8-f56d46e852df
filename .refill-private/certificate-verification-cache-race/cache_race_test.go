package certificateverificationcacherace

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"seed-vigor-gate/internal/domain"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"seed-vigor-gate/internal/web"
	"sync"
	"testing"
)

type gatedRepository struct {
	mu      sync.Mutex
	entered int
	both    chan struct{}
	release chan struct{}
}

func newGatedRepository() *gatedRepository {
	return &gatedRepository{both: make(chan struct{}), release: make(chan struct{})}
}

func (r *gatedRepository) LoadCertificateMaterial(_ context.Context, number string) (store.CertificateMaterial, error) {
	r.mu.Lock()
	r.entered++
	if r.entered == 2 {
		close(r.both)
	}
	r.mu.Unlock()
	<-r.release
	return store.CertificateMaterial{Certificate: domain.QualificationCertificate{CertificateNo: number}}, nil
}

func (r *gatedRepository) Load(context.Context, string) (*domain.Aggregate, error) {
	return nil, errors.New("unexpected Load")
}

func (r *gatedRepository) Append(context.Context, string, int64, string, domain.Event, string) (store.Receipt, error) {
	return store.Receipt{}, errors.New("unexpected Append")
}

func (r *gatedRepository) LookupReceipt(context.Context, string) (store.Receipt, bool) {
	return store.Receipt{}, false
}

func (r *gatedRepository) ChainDigest(context.Context, string) (string, error) {
	return "", errors.New("unexpected ChainDigest")
}

func (r *gatedRepository) FindCertificate(context.Context, string) (domain.QualificationCertificate, error) {
	return domain.QualificationCertificate{}, errors.New("unexpected FindCertificate")
}

func TestConcurrentCertificateVerificationCacheIsSafe(t *testing.T) {
	repository := newGatedRepository()
	service := qualification.NewService(repository, protocol.NewEngine())
	server := httptest.NewServer(web.NewHandler(service))
	defer server.Close()

	start := make(chan struct{})
	statuses := make(chan int, 2)
	errorsSeen := make(chan error, 2)
	var requests sync.WaitGroup
	for _, number := range []string{"CERT-CACHE-A", "CERT-CACHE-B"} {
		requests.Add(1)
		go func(certificateNo string) {
			defer requests.Done()
			<-start
			response, err := http.Get(server.URL + "/api/certificates/" + certificateNo + "/verify")
			if err != nil {
				errorsSeen <- err
				return
			}
			defer response.Body.Close()
			statuses <- response.StatusCode
		}(number)
	}

	close(start)
	<-repository.both
	close(repository.release)
	requests.Wait()
	close(errorsSeen)
	close(statuses)

	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("并发凭据校验请求失败: %v", err)
		}
	}
	count := 0
	for status := range statuses {
		count++
		if status != http.StatusOK {
			t.Fatalf("凭据校验状态码 = %d, want %d", status, http.StatusOK)
		}
	}
	if count != 2 {
		t.Fatalf("完成请求数 = %d, want 2", count)
	}
}
