package viewcachealias_test

import (
	"context"
	"seed-vigor-gate/internal/domain"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"testing"
)

func TestCachedAssessmentViewCannotBePoisoned(t *testing.T) {
	ledger, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	service := qualification.NewService(ledger, protocol.NewEngine())
	ctx := context.Background()
	const assessmentID = "cache-alias-assessment"
	_, err = service.Create(ctx, "cache-alias-create", qualification.CreateCommand{
		ID:                   assessmentID,
		LotCode:              "LOT-CACHE",
		SpeciesName:          "水稻",
		HarvestYear:          2025,
		SubmittedQuantity:    100,
		PretreatmentBoundary: "清水浸种",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.Get(ctx, assessmentID)
	if err != nil {
		t.Fatal(err)
	}
	first.Observations["phantom-replicate"] = []domain.Observation{{
		ID:               "phantom-observation",
		ReplicateID:      "phantom-replicate",
		DayNo:            7,
		NormalGerminated: 100,
		RecordedBy:       "调用方本地补充",
		RevisionNo:       1,
	}}

	persisted, err := ledger.Load(ctx, assessmentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := persisted.Observations["phantom-replicate"]; exists {
		t.Fatal("测试前置条件无效：调用方修改不应写入 Ledger")
	}

	second, err := service.Get(ctx, assessmentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := second.Observations["phantom-replicate"]; exists {
		t.Fatalf("TestCachedAssessmentViewCannotBePoisoned: 同版本缓存复用了首次返回视图的嵌套 map，第二次查询出现未持久化观测")
	}
}
