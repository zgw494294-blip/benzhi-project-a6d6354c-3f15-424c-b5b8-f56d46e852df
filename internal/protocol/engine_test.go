package protocol

import (
	"fmt"
	"seed-vigor-gate/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestValidateReturnsStableStructuredIssues(t *testing.T) {
	issues := Validate(domain.ProtocolSnapshot{ReplicateCount: 1, SeedsPerReplicate: 100, TemperatureMinC: 30, TemperatureMaxC: 20, ObservationDays: []int{7, 3}, TerminationDay: 5, MinimumGerminationRate: 101, MaximumDispersion: -1}, 50)
	if len(issues) < 6 {
		t.Fatalf("完整性问题不足: %+v", issues)
	}
	for i := 1; i < len(issues); i++ {
		if issues[i-1].Field > issues[i].Field {
			t.Fatalf("问题未稳定排序: %+v", issues)
		}
	}
}

func TestCalculateUsesLatestRevisionAndEffectiveReplicates(t *testing.T) {
	a := domain.EmptyAggregate()
	a.Protocol = &domain.ProtocolSnapshot{ReplicateCount: 2, SeedsPerReplicate: 50, ObservationDays: []int{3, 7}, TerminationDay: 7, MinimumGerminationRate: 80, MaximumDispersion: 10}
	a.Replicates["r1"] = domain.Replicate{ID: "r1", Label: "R1", SownQuantity: 50, Status: domain.ReplicateComplete}
	a.Replicates["r2"] = domain.Replicate{ID: "r2", Label: "R2", SownQuantity: 50, Status: domain.ReplicateComplete}
	a.Observations["r1"] = []domain.Observation{{DayNo: 3, NormalGerminated: 38, RevisionNo: 1}, {DayNo: 3, NormalGerminated: 42, RevisionNo: 2}, {DayNo: 7, NormalGerminated: 46, RevisionNo: 1}}
	a.Observations["r2"] = []domain.Observation{{DayNo: 3, NormalGerminated: 40, RevisionNo: 1}, {DayNo: 7, NormalGerminated: 44, RevisionNo: 1}}
	metrics, err := NewEngine().Calculate(a, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if metrics.FinalGerminationRate != 90 || metrics.Dispersion != 4 || metrics.ThresholdDay != 3 || metrics.Decision != "qualified" {
		t.Fatalf("指标不正确: %+v", metrics)
	}
}

func TestProgressUsesLatestRevisionAndFlagsOutlier(t *testing.T) {
	a := domain.EmptyAggregate()
	a.Protocol = &domain.ProtocolSnapshot{ObservationDays: []int{3, 5, 7}}
	for index, rate := range []int{40, 41, 39, 10} {
		id := fmt.Sprintf("r%d", index+1)
		a.Replicates[id] = domain.Replicate{ID: id, Label: strings.ToUpper(id), SownQuantity: 50, Status: domain.ReplicateActive}
		a.Observations[id] = []domain.Observation{{DayNo: 3, NormalGerminated: rate - 1, RevisionNo: 1}, {DayNo: 3, NormalGerminated: rate, RevisionNo: 2}}
	}
	projection := BuildProgress(a)
	if len(projection.Days) != 3 || projection.Days[0].CoveredGroups != 4 || projection.Days[0].AverageNormalRate == nil {
		t.Fatalf("进程覆盖计算错误: %+v", projection)
	}
	if len(projection.Days[0].Outliers) != 1 || projection.Days[0].Outliers[0].ReplicateID != "r4" || projection.Days[0].Outliers[0].RevisionNo != 2 {
		t.Fatalf("离群提示未使用最新修订: %+v", projection.Days[0].Outliers)
	}
	if len(projection.Days[1].MissingReplicateIDs) != 4 {
		t.Fatalf("缺失日不应补零: %+v", projection.Days[1])
	}
}
