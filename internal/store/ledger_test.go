package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"seed-vigor-gate/internal/domain"
	"strings"
	"testing"
	"time"
)

func createEvent(t *testing.T) domain.Event {
	t.Helper()
	event, err := domain.CreateAssessment(domain.CreateAssessmentInput{ID: "a1", LotCode: "L1", SpeciesName: "玉米", HarvestYear: 2025, SubmittedQuantity: 100, PretreatmentBoundary: "清水"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestLedgerDurabilityIdempotencyAndRecovery(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	ledger, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ledger.Append(ctx, "a1", 0, "key-1", createEvent(t), "")
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := ledger.Append(ctx, "a1", 0, "key-1", createEvent(t), "")
	if err != nil || !replayed.Replayed || replayed.Version != receipt.Version {
		t.Fatalf("幂等回执错误: %+v %v", replayed, err)
	}
	_, err = ledger.Append(ctx, "a1", 0, "key-2", createEvent(t), "")
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != domain.CodeConflict {
		t.Fatalf("预期版本冲突，得到 %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	aggregate, err := reopened.Load(ctx, "a1")
	if err != nil || aggregate.Assessment.Version != 1 {
		t.Fatalf("恢复失败: %+v %v", aggregate, err)
	}
}

func TestLedgerRejectsTamperedHashChain(t *testing.T) {
	dir := t.TempDir()
	ledger, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Append(context.Background(), "a1", 0, "key-1", createEvent(t), ""); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(b), "L1", "L2", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil || !strings.Contains(err.Error(), "摘要不匹配") {
		t.Fatalf("篡改账本应被拒绝，得到 %v", err)
	}
}
