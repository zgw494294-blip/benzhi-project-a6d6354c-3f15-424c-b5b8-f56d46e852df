package qualification

import (
	"fmt"
	"seed-vigor-gate/internal/domain"
)

type ChecklistItem struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Complete bool   `json:"complete"`
	Blocking bool   `json:"blocking"`
	Detail   string `json:"detail"`
}

func BuildChecklist(a *domain.Aggregate) []ChecklistItem {
	return []ChecklistItem{
		{Code: "BATCH", Label: "批次和样本边界已登记", Complete: a.Assessment.ID != "", Blocking: true, Detail: fmt.Sprintf("批次 %s，送检 %d 粒", a.Assessment.LotCode, a.Assessment.SubmittedQuantity)},
		{Code: "PROTOCOL", Label: "试验方案已冻结", Complete: a.Protocol != nil, Blocking: true, Detail: protocolDetail(a)},
		{Code: "REPLICATES", Label: "原始重复组已完整布置", Complete: originalReplicatesReady(a), Blocking: true, Detail: replicateDetail(a)},
		{Code: "TERMINAL_DATA", Label: "有效组终止日数据齐备", Complete: terminalDataReady(a), Blocking: true, Detail: terminalDetail(a)},
		{Code: "DEVIATIONS", Label: "异常处置均已闭环", Complete: !a.HasOpenDeviation(), Blocking: true, Detail: deviationDetail(a)},
		{Code: "METRICS", Label: "资格规则已经计算", Complete: a.Metrics != nil, Blocking: true, Detail: metricsDetail(a)},
		{Code: "REVIEW_ITEMS", Label: "复核退回事项均已解决", Complete: reviewItemsReady(a), Blocking: true, Detail: reviewItemsDetail(a)},
		{Code: "REVIEW", Label: "复核结论已经批准", Complete: a.Assessment.Status == domain.StatusApproved || a.Assessment.Status == domain.StatusSealed, Blocking: true, Detail: reviewDetail(a)},
		{Code: "CERTIFICATE", Label: "不可变资格凭据已封存", Complete: a.Certificate != nil, Blocking: false, Detail: certificateDetail(a)},
	}
}

func reviewItemsReady(a *domain.Aggregate) bool {
	for _, item := range a.ReviewItems {
		if !item.Resolved {
			return false
		}
	}
	return true
}

func reviewItemsDetail(a *domain.Aggregate) string {
	open := 0
	for _, item := range a.ReviewItems {
		if !item.Resolved {
			open++
		}
	}
	return fmt.Sprintf("退回事项共 %d 项，未解决 %d 项", len(a.ReviewItems), open)
}

func originalReplicatesReady(a *domain.Aggregate) bool {
	if a.Protocol == nil {
		return false
	}
	count := 0
	for _, replicate := range a.Replicates {
		if replicate.Kind == domain.ReplicateOriginal {
			count++
		}
	}
	return count == a.Protocol.ReplicateCount
}

func terminalDataReady(a *domain.Aggregate) bool {
	if a.Protocol == nil || len(a.Replicates) == 0 {
		return false
	}
	active := 0
	for id, replicate := range a.Replicates {
		if replicate.Status == domain.ReplicateVoid {
			continue
		}
		active++
		if !a.HasTerminalObservation(id) {
			return false
		}
	}
	return active > 0
}

func protocolDetail(a *domain.Aggregate) string {
	if a.Protocol == nil {
		return "尚未冻结"
	}
	return fmt.Sprintf("%d 组 × %d 粒，终止日 D%d", a.Protocol.ReplicateCount, a.Protocol.SeedsPerReplicate, a.Protocol.TerminationDay)
}

func replicateDetail(a *domain.Aggregate) string {
	original, retest := 0, 0
	for _, replicate := range a.Replicates {
		if replicate.Kind == domain.ReplicateOriginal {
			original++
		} else {
			retest++
		}
	}
	return fmt.Sprintf("原始组 %d，定向复测组 %d", original, retest)
}

func terminalDetail(a *domain.Aggregate) string {
	if a.Protocol == nil {
		return "等待冻结方案"
	}
	complete, valid := 0, 0
	for id, replicate := range a.Replicates {
		if replicate.Status == domain.ReplicateVoid {
			continue
		}
		valid++
		if a.HasTerminalObservation(id) {
			complete++
		}
	}
	return fmt.Sprintf("%d/%d 个有效组已完成 D%d", complete, valid, a.Protocol.TerminationDay)
}

func deviationDetail(a *domain.Aggregate) string {
	open := 0
	for _, deviation := range a.Deviations {
		if deviation.Status == domain.DeviationOpen {
			open++
		}
	}
	return fmt.Sprintf("异常共 %d 项，未闭环 %d 项", len(a.Deviations), open)
}

func metricsDetail(a *domain.Aggregate) string {
	if a.Metrics == nil {
		return "等待有效组终止日数据"
	}
	return fmt.Sprintf("建议结论 %s，发芽率 %.2f%%", a.Metrics.Decision, a.Metrics.FinalGerminationRate)
}

func reviewDetail(a *domain.Aggregate) string {
	if len(a.Reviews) == 0 {
		return "尚无复核记录"
	}
	last := a.Reviews[len(a.Reviews)-1]
	if last.Approved {
		return "由 " + last.ReviewedBy + " 批准"
	}
	return "由 " + last.ReviewedBy + " 退回：" + last.Reason
}

func certificateDetail(a *domain.Aggregate) string {
	if a.Certificate == nil {
		return "批准后可执行一次性封存"
	}
	return "凭据编号 " + a.Certificate.CertificateNo
}
