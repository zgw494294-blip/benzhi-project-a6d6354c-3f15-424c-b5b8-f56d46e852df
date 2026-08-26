package protocol

import (
	"fmt"
	"seed-vigor-gate/internal/domain"
	"sort"
)

func Evaluate(snapshot domain.ProtocolSnapshot, calc Calculation, aggregate *domain.Aggregate) []domain.RuleHit {
	hits := []domain.RuleHit{
		{Code: "FINAL_RATE", Passed: calc.FinalRate >= snapshot.MinimumGerminationRate, Blocking: true, Message: fmt.Sprintf("最终发芽率 %.2f%%，要求不低于 %.2f%%", calc.FinalRate, snapshot.MinimumGerminationRate)},
		{Code: "DISPERSION", Passed: calc.Dispersion <= snapshot.MaximumDispersion, Blocking: true, Message: fmt.Sprintf("重复组极差 %.2f，要求不高于 %.2f", calc.Dispersion, snapshot.MaximumDispersion)},
		{Code: "THRESHOLD_DAY", Passed: calc.ThresholdDay > 0 && calc.ThresholdDay <= snapshot.TerminationDay, Blocking: true, Message: thresholdMessage(calc.ThresholdDay, snapshot.TerminationDay)},
		{Code: "DEVIATION_CLOSED", Passed: !aggregate.HasOpenDeviation(), Blocking: true, Message: deviationMessage(aggregate.HasOpenDeviation())},
		{Code: "ACTIVE_REPLICATES", Passed: calc.ActiveReplicateCount >= snapshot.ReplicateCount, Blocking: true, Message: fmt.Sprintf("有效重复组 %d 组，方案要求至少 %d 组", calc.ActiveReplicateCount, snapshot.ReplicateCount)},
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Code < hits[j].Code })
	return hits
}

func thresholdMessage(day, termination int) string {
	if day == 0 {
		return fmt.Sprintf("截至终止日第 %d 天仍未达到最低发芽率", termination)
	}
	return fmt.Sprintf("第 %d 天首次达到最低发芽率，终止日为第 %d 天", day, termination)
}

func deviationMessage(open bool) string {
	if open {
		return "存在尚未完成定向复测的异常"
	}
	return "所有异常均已完成定向复测并闭环"
}

func Decision(hits []domain.RuleHit) string {
	for _, hit := range hits {
		if hit.Blocking && !hit.Passed {
			return "not_qualified"
		}
	}
	return "qualified"
}
