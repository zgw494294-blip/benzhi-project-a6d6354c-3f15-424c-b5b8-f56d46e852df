package ledgercloneevidencealias

import (
	"bytes"
	"context"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"testing"
	"time"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

func TestLedgerLoadCloneCannotCorruptDurableState(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	ledger, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	clock := fixedClock{now: time.Date(2026, time.August, 26, 9, 0, 0, 0, time.UTC)}
	service := qualification.NewServiceWithClock(ledger, protocol.NewEngine(), clock)
	receipt, err := service.Create(ctx, "clone-create", qualification.CreateCommand{
		ID:                   "clone-assessment",
		LotCode:              "LOT-CLONE",
		SpeciesName:          "水稻",
		HarvestYear:          2025,
		SubmittedQuantity:    200,
		PretreatmentBoundary: "清水浸种",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.FreezeProtocol(ctx, "clone-assessment", "clone-freeze", qualification.FreezeProtocolCommand{
		Versioned:              qualification.Versioned{ExpectedVersion: receipt.Version},
		ReplicateCount:         2,
		SeedsPerReplicate:      50,
		TemperatureMinC:        20,
		TemperatureMaxC:        30,
		ObservationDays:        []int{3, 7},
		TerminationDay:         7,
		MinimumGerminationRate: 80,
		MaximumDispersion:      10,
	})
	if err != nil {
		t.Fatal(err)
	}

	external, err := ledger.Load(ctx, "clone-assessment")
	if err != nil {
		t.Fatal(err)
	}
	originalAudit := append([]byte(nil), external.Audit[0].Data...)
	external.Protocol.MinimumGerminationRate = 1
	external.Protocol.ObservationDays[0] = 99
	external.Audit[0].Data[0] = '['

	live, err := ledger.Load(ctx, "clone-assessment")
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, err := reopened.Load(ctx, "clone-assessment")
	if err != nil {
		t.Fatal(err)
	}

	if live.Protocol.MinimumGerminationRate != recovered.Protocol.MinimumGerminationRate ||
		live.Protocol.ObservationDays[0] != recovered.Protocol.ObservationDays[0] ||
		!bytes.Equal(live.Audit[0].Data, originalAudit) {
		t.Fatalf("Ledger.Load 返回值污染了进程内证据；live=%v/%v/%q recovered=%v/%v/%q",
			live.Protocol.MinimumGerminationRate,
			live.Protocol.ObservationDays,
			live.Audit[0].Data,
			recovered.Protocol.MinimumGerminationRate,
			recovered.Protocol.ObservationDays,
			recovered.Audit[0].Data,
		)
	}
}
