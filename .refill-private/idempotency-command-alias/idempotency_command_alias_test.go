package idempotency_command_alias_test

import (
	"context"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"testing"
)

func TestDifferentCommandCannotReplayCreateReceipt(t *testing.T) {
	ledger, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	service := qualification.NewService(ledger, protocol.NewEngine())
	ctx := context.Background()
	receipt, err := service.Create(ctx, "shared-key", qualification.CreateCommand{
		ID: "idem-a", LotCode: "LOT-A", SpeciesName: "水稻", HarvestYear: 2025,
		SubmittedQuantity: 100, PretreatmentBoundary: "清水",
	})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.FreezeProtocol(ctx, "idem-a", "shared-key", qualification.FreezeProtocolCommand{
		Versioned: qualification.Versioned{ExpectedVersion: receipt.Version},
		ReplicateCount: 2, SeedsPerReplicate: 25, TemperatureMinC: 20,
		TemperatureMaxC: 30, ObservationDays: []int{3, 7}, TerminationDay: 7,
		MinimumGerminationRate: 80, MaximumDispersion: 10,
	})
	if err == nil && replayed.EventType == "assessment.created" {
		t.Fatalf("TestDifferentCommandCannotReplayCreateReceipt: freeze silently replayed an unrelated create receipt: %+v", replayed)
	}
}
