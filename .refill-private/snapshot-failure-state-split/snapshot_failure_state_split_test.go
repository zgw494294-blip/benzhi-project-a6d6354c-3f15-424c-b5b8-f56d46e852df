package snapshot_failure_state_split_test

import (
	"context"
	"os"
	"path/filepath"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"testing"
)

func TestSnapshotFailureCannotHideDurableCommit(t *testing.T) {
	dir := t.TempDir()
	ledger, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := os.Mkdir(snapshotPath, 0o750); err != nil {
		t.Fatal(err)
	}

	service := qualification.NewService(ledger, protocol.NewEngine())
	_, appendErr := service.Create(context.Background(), "snapshot-failure-create", qualification.CreateCommand{
		ID:                   "snapshot-failure-assessment",
		LotCode:              "LOT-SNAPSHOT",
		SpeciesName:          "水稻",
		HarvestYear:          2025,
		SubmittedQuantity:    200,
		PretreatmentBoundary: "清水浸种",
	})
	if appendErr == nil {
		t.Fatal("快照资源失效应使当前实现返回错误")
	}

	_, liveErr := ledger.Load(context.Background(), "snapshot-failure-assessment")
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, recoveryErr := reopened.Load(context.Background(), "snapshot-failure-assessment")
	if recoveryErr != nil {
		t.Fatalf("账本已同步的事件应能在重启后恢复: %v", recoveryErr)
	}
	if liveErr != nil {
		t.Fatalf("同一笔已同步事件在进程内不可见、重启后却恢复为版本 %d: %v", recovered.Assessment.Version, liveErr)
	}
}
