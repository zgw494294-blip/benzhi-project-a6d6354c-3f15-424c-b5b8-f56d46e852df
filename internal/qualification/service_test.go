package qualification

import (
	"context"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/store"
	"testing"
)

func intPointer(value int) *int { return &value }

func TestBatchObservationReceiptAndIdempotency(t *testing.T) {
	ledger, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	service := NewService(ledger, protocol.NewEngine())
	ctx := context.Background()
	receipt, err := service.Create(ctx, "create", CreateCommand{ID: "batch-a", LotCode: "L", SpeciesName: "水稻", HarvestYear: 2025, SubmittedQuantity: 200, PretreatmentBoundary: "清水"})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = service.FreezeProtocol(ctx, "batch-a", "freeze", FreezeProtocolCommand{Versioned: Versioned{ExpectedVersion: receipt.Version}, ReplicateCount: 2, SeedsPerReplicate: 50, TemperatureMinC: 20, TemperatureMaxC: 30, ObservationDays: []int{3, 7}, TerminationDay: 7, MinimumGerminationRate: 80, MaximumDispersion: 10})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = service.PlaceReplicates(ctx, "batch-a", "place", PlaceReplicatesCommand{Versioned: Versioned{ExpectedVersion: receipt.Version}, Replicates: []ReplicateInput{{ID: "r1", Label: "R1", SownQuantity: 50}, {ID: "r2", Label: "R2", SownQuantity: 50}}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = service.Start(ctx, "batch-a", "start", StartCommand{Versioned{ExpectedVersion: receipt.Version}})
	if err != nil {
		t.Fatal(err)
	}
	batch := RecordObservationBatchCommand{Versioned: Versioned{ExpectedVersion: receipt.Version}, DayNo: 3, RecordedBy: "技术员", Observations: []BatchObservationInput{
		{ReplicateID: "r1", NormalGerminated: intPointer(40), AbnormalSeedlings: intPointer(2), HardSeeds: intPointer(1), DeadSeeds: intPointer(1), UngerminatedSeeds: intPointer(6)},
		{ReplicateID: "r2", NormalGerminated: intPointer(41), AbnormalSeedlings: intPointer(2), HardSeeds: intPointer(1), DeadSeeds: intPointer(1), UngerminatedSeeds: intPointer(5)},
	}}
	before := receipt.Version
	receipt, err = service.RecordObservationBatch(ctx, "batch-a", "batch-day-3", batch)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Version != before+1 || receipt.SuccessfulGroups != 2 || receipt.TerminalReady {
		t.Fatalf("批量回执错误: %+v", receipt)
	}
	replayed, err := service.RecordObservationBatch(ctx, "batch-a", "batch-day-3", batch)
	if err != nil || !replayed.Replayed || replayed.Version != receipt.Version || replayed.SuccessfulGroups != 2 {
		t.Fatalf("批量幂等回执错误: %+v %v", replayed, err)
	}
	batch.ExpectedVersion = receipt.Version
	batch.DayNo = 7
	batch.Observations[0].NormalGerminated = intPointer(46)
	batch.Observations[0].UngerminatedSeeds = intPointer(0)
	batch.Observations[1].NormalGerminated = intPointer(45)
	batch.Observations[1].UngerminatedSeeds = intPointer(1)
	receipt, err = service.RecordObservationBatch(ctx, "batch-a", "batch-day-7", batch)
	if err != nil || !receipt.TerminalReady {
		t.Fatalf("终止日批次错误: %+v %v", receipt, err)
	}
	receipt, err = service.Calculate(ctx, "batch-a", "calculate", CalculateCommand{Versioned{ExpectedVersion: receipt.Version}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = service.ApproveReview(ctx, "batch-a", "approve", ReviewCommand{Versioned: Versioned{ExpectedVersion: receipt.Version}, Reviewer: "复核员", Reason: "证据完整"})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = service.Seal(ctx, "batch-a", "seal", SealCommand{Versioned{ExpectedVersion: receipt.Version}})
	if err != nil {
		t.Fatal(err)
	}
	verification, err := service.VerifyCertificate(ctx, receipt.CertificateNo)
	if err != nil {
		t.Fatal(err)
	}
	wantCodes := []string{"CERTIFICATE_DIGEST", "PROTOCOL_DIGEST", "INPUT_DIGEST", "EVENT_CHAIN", "REVIEW_MATERIAL"}
	if !verification.Valid || len(verification.Items) != len(wantCodes) {
		t.Fatalf("凭据报告无效: %+v", verification)
	}
	for index, code := range wantCodes {
		if verification.Items[index].Code != code || !verification.Items[index].Passed {
			t.Fatalf("凭据校验项顺序或结果错误: %+v", verification.Items)
		}
	}
}
