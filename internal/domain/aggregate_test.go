package domain

import (
	"errors"
	"testing"
	"time"
)

var testTime = time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)

func preparedAggregate(t *testing.T) *Aggregate {
	t.Helper()
	created, err := CreateAssessment(CreateAssessmentInput{ID: "a1", LotCode: "L1", SpeciesName: "水稻", HarvestYear: 2025, SubmittedQuantity: 200, PretreatmentBoundary: "清水"}, testTime)
	if err != nil {
		t.Fatal(err)
	}
	a := EmptyAggregate()
	if err := a.Apply(created); err != nil {
		t.Fatal(err)
	}
	protocol := ProtocolSnapshot{ReplicateCount: 2, SeedsPerReplicate: 50, TemperatureMinC: 20, TemperatureMaxC: 30, ObservationDays: []int{3, 7}, TerminationDay: 7, MinimumGerminationRate: 80, MaximumDispersion: 10, ContentDigest: "digest"}
	frozen, err := a.FreezeProtocol(protocol, testTime.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(frozen); err != nil {
		t.Fatal(err)
	}
	placed, err := a.PlaceReplicates([]Replicate{{ID: "r1", Label: "R1", SownQuantity: 50}, {ID: "r2", Label: "R2", SownQuantity: 50}}, testTime.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(placed); err != nil {
		t.Fatal(err)
	}
	started, err := a.StartObservation(testTime.Add(3 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(started); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestObservationConservationSequenceAndRevision(t *testing.T) {
	a := preparedAggregate(t)
	_, err := a.RecordObservation(ObservationInput{ID: "bad", ReplicateID: "r1", DayNo: 3, NormalGerminated: 40, UngerminatedSeeds: 9, RecordedBy: "技术员"}, testTime)
	if err == nil {
		t.Fatal("计数不守恒应被拒绝")
	}
	_, err = a.RecordObservation(ObservationInput{ID: "skip", ReplicateID: "r1", DayNo: 7, NormalGerminated: 45, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 1, RecordedBy: "技术员"}, testTime)
	if err == nil {
		t.Fatal("跳过首个观察日应被拒绝")
	}
	first := ObservationInput{ID: "o1", ReplicateID: "r1", DayNo: 3, NormalGerminated: 40, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 6, RecordedBy: "技术员"}
	event, err := a.RecordObservation(first, testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(event); err != nil {
		t.Fatal(err)
	}
	first.NormalGerminated = 41
	first.UngerminatedSeeds = 5
	revision, err := a.RecordObservation(first, testTime.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(revision); err != nil {
		t.Fatal(err)
	}
	latest := LatestObservations(a.Observations["r1"])[3]
	if latest.RevisionNo != 2 || latest.NormalGerminated != 41 {
		t.Fatalf("修订轨迹不正确: %+v", latest)
	}
}

func TestDeviationCreatesOnlyAffectedRetestAndRequiresCompletion(t *testing.T) {
	a := preparedAggregate(t)
	event, err := a.RegisterDeviation(RegisterDeviationInput{ID: "d1", Category: "contamination", AffectedReplicateIDs: []string{"r1"}, Description: "污染", Disposition: "复测"}, testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(event); err != nil {
		t.Fatal(err)
	}
	if len(a.Replicates) != 3 || a.Replicates["r1"].Status != ReplicateVoid {
		t.Fatalf("定向复测范围错误: %+v", a.Replicates)
	}
	_, err = a.CloseDeviation("d1", testTime)
	if err == nil {
		t.Fatal("复测未完成时不能闭环")
	}
}

func TestVersionAndStateErrorsAreTyped(t *testing.T) {
	a := preparedAggregate(t)
	_, err := a.StartObservation(testTime)
	var typed *DomainError
	if !errors.As(err, &typed) || typed.Code != CodeState {
		t.Fatalf("需要状态冲突错误，得到 %v", err)
	}
	conflict := VersionConflict(a.Assessment.Version)
	if !errors.As(conflict, &typed) || typed.CurrentVersion != a.Assessment.Version {
		t.Fatalf("版本错误缺少当前版本: %v", conflict)
	}
}

func TestBatchObservationIsAtomicAndAdvancesOneVersion(t *testing.T) {
	a := preparedAggregate(t)
	before := a.Assessment.Version
	invalid := ObservationBatchInput{DayNo: 3, RecordedBy: "技术员", Observations: []ObservationInput{
		{ID: "b1", ReplicateID: "r1", NormalGerminated: 40, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 6},
		{ID: "b2", ReplicateID: "r2", NormalGerminated: 40, UngerminatedSeeds: 9},
	}}
	_, issues, err := a.RecordObservationBatch(invalid, testTime)
	if err != nil || len(issues) == 0 {
		t.Fatalf("无效批次应返回逐组问题: %+v %v", issues, err)
	}
	if a.Assessment.Version != before || len(a.Observations) != 0 {
		t.Fatal("预校验失败不能改变聚合")
	}
	invalid.Observations[1] = ObservationInput{ID: "b2", ReplicateID: "r2", NormalGerminated: 41, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 5}
	event, issues, err := a.RecordObservationBatch(invalid, testTime)
	if err != nil || len(issues) != 0 {
		t.Fatalf("有效批次被拒绝: %+v %v", issues, err)
	}
	if err := a.Apply(event); err != nil {
		t.Fatal(err)
	}
	if a.Assessment.Version != before+1 || len(a.Observations["r1"]) != 1 || len(a.Observations["r2"]) != 1 {
		t.Fatalf("批量事件未原子推进: v%d %+v", a.Assessment.Version, a.Observations)
	}
}

func TestDeviationSampleBoundaryAndFullScheduleReadiness(t *testing.T) {
	a := preparedAggregate(t)
	a.Assessment.SubmittedQuantity = 150
	_, err := a.RegisterDeviation(RegisterDeviationInput{ID: "too-many", Category: "temperature_interruption", AffectedReplicateIDs: []string{"r1", "r2"}, Description: "中断", Disposition: "复测"}, testTime)
	if err == nil || a.Replicates["r1"].Status == ReplicateVoid {
		t.Fatal("余量不足时必须整批拒绝且不能作废原组")
	}
	a.Assessment.SubmittedQuantity = 200
	event, err := a.RegisterDeviation(RegisterDeviationInput{ID: "d-ready", Category: "temperature_interruption", AffectedReplicateIDs: []string{"r1"}, Description: "中断", Disposition: "复测"}, testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(event); err != nil {
		t.Fatal(err)
	}
	retest := "d-ready-retest-r1"
	terminal := ObservationInput{ID: "terminal", ReplicateID: retest, DayNo: 7, NormalGerminated: 45, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 1, RecordedBy: "技术员"}
	if _, err := a.RecordObservation(terminal, testTime); err == nil {
		t.Fatal("复测组不能跳过中间观察日")
	}
	for _, input := range []ObservationInput{
		{ID: "d3", ReplicateID: retest, DayNo: 3, NormalGerminated: 40, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 6, RecordedBy: "技术员"},
		terminal,
	} {
		e, recordErr := a.RecordObservation(input, testTime)
		if recordErr != nil {
			t.Fatal(recordErr)
		}
		if err := a.Apply(e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.CloseDeviation("d-ready", testTime); err == nil {
		t.Fatal("当前指标未重算时不能闭环")
	}
	metricsEvent, _ := NewEvent(EventMetricsCalculated, testTime, MetricsCalculatedData{Metrics: Metrics{SourceVersion: a.Assessment.Version}})
	if err := a.Apply(metricsEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CloseDeviation("d-ready", testTime); err != nil {
		t.Fatalf("全日程齐备且指标当前时应允许闭环: %v", err)
	}
}

func TestReviewItemsRequireChangedAndRecalculatedEvidence(t *testing.T) {
	a := preparedAggregate(t)
	for _, input := range []ObservationInput{
		{ID: "r1d3", ReplicateID: "r1", DayNo: 3, NormalGerminated: 40, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 6, RecordedBy: "技术员"},
		{ID: "r2d3", ReplicateID: "r2", DayNo: 3, NormalGerminated: 41, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 5, RecordedBy: "技术员"},
		{ID: "r1d7", ReplicateID: "r1", DayNo: 7, NormalGerminated: 45, AbnormalSeedlings: 2, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 1, RecordedBy: "技术员"},
		{ID: "r2d7", ReplicateID: "r2", DayNo: 7, NormalGerminated: 46, AbnormalSeedlings: 1, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 1, RecordedBy: "技术员"},
	} {
		event, err := a.RecordObservation(input, testTime)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.Apply(event); err != nil {
			t.Fatal(err)
		}
	}
	metricsEvent, _ := NewEvent(EventMetricsCalculated, testTime, MetricsCalculatedData{Metrics: Metrics{SourceVersion: a.Assessment.Version}})
	if err := a.Apply(metricsEvent); err != nil {
		t.Fatal(err)
	}
	returned, err := a.ReturnReviewWithItems("复核员", "补充观测依据", []ReviewItemInput{{ID: "item-1", EvidenceArea: EvidenceObservation, Problem: "R1 数据需核对", Requirement: "修订并重新计算", ReplicateIDs: []string{"r1"}}}, testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(returned); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveReviewItem("item-1", "已经核对", testTime); err == nil {
		t.Fatal("证据未变化时不能解决事项")
	}
	revision := ObservationInput{ID: "r1d7-r2", ReplicateID: "r1", DayNo: 7, NormalGerminated: 46, AbnormalSeedlings: 1, HardSeeds: 1, DeadSeeds: 1, UngerminatedSeeds: 1, RecordedBy: "技术员"}
	revisionEvent, err := a.RecordObservation(revision, testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(revisionEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveReviewItem("item-1", "已经核对", testTime); err == nil {
		t.Fatal("修订后未重新计算时不能解决事项")
	}
	recalculated, _ := NewEvent(EventMetricsCalculated, testTime, MetricsCalculatedData{Metrics: Metrics{SourceVersion: a.Assessment.Version}})
	if err := a.Apply(recalculated); err != nil {
		t.Fatal(err)
	}
	resolved, err := a.ResolveReviewItem("item-1", "已修订 R1 终止日数据并重新计算", testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(resolved); err != nil {
		t.Fatal(err)
	}
	resubmitted, err := a.ResubmitReview("技术员", testTime)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(resubmitted); err != nil {
		t.Fatal(err)
	}
	if a.Assessment.Status != StatusPendingReview || !a.ReviewItems["item-1"].Resolved || a.Reviews[len(a.Reviews)-1].Items[0].Resolved {
		t.Fatalf("再次送审状态或不可改写退回记录错误: %+v", a)
	}
}
