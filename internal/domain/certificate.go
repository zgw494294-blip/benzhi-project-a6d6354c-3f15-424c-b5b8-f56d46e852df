package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ReviewItemInput struct {
	ID           string
	EvidenceArea ReviewEvidenceArea
	Problem      string
	Requirement  string
	ReplicateIDs []string
}

func (a *Aggregate) ReturnReview(reviewer, reason string, at time.Time) (Event, error) {
	return a.ReturnReviewWithItems(reviewer, reason, nil, at)
}

func (a *Aggregate) ReturnReviewWithItems(reviewer, reason string, items []ReviewItemInput, at time.Time) (Event, error) {
	if a.Assessment.Status != StatusPendingReview {
		return Event{}, State("pending_review", a.Assessment.Status)
	}
	if strings.TrimSpace(reviewer) == "" || strings.TrimSpace(reason) == "" {
		return Event{}, Invalid("复核人和退回理由不能为空", nil)
	}
	if len(items) == 0 {
		return Event{}, Invalid("退回时至少登记一个结构化补充事项", map[string]string{"items": "至少一项"})
	}
	allowed := map[ReviewEvidenceArea]bool{EvidenceProtocol: true, EvidenceObservation: true, EvidenceDeviation: true, EvidenceRules: true}
	blockingRules := make([]string, 0)
	if a.Metrics != nil {
		for _, hit := range a.Metrics.RuleHits {
			if hit.Blocking && !hit.Passed {
				blockingRules = append(blockingRules, hit.Code)
			}
		}
	}
	result := make([]ReviewItem, 0, len(items))
	seen := map[string]bool{}
	for index, input := range items {
		prefix := fmt.Sprintf("items.%d", index)
		if strings.TrimSpace(input.ID) == "" || seen[input.ID] {
			return Event{}, Invalid("退回事项编号不能为空且不能重复", map[string]string{prefix + ".id": "无效编号"})
		}
		seen[input.ID] = true
		if !allowed[input.EvidenceArea] {
			return Event{}, Invalid("证据区域不受支持", map[string]string{prefix + ".evidenceArea": "仅支持 protocol、observation、deviation、rules"})
		}
		if strings.TrimSpace(input.Problem) == "" || strings.TrimSpace(input.Requirement) == "" {
			return Event{}, Invalid("问题说明和补充要求不能为空", map[string]string{prefix: "内容不完整"})
		}
		replicates := make([]string, 0, len(input.ReplicateIDs))
		replicateSeen := map[string]bool{}
		for _, id := range input.ReplicateIDs {
			if _, ok := a.Replicates[id]; !ok {
				return Event{}, Invalid("退回事项关联了未知重复组", map[string]string{prefix + ".replicateIds": id})
			}
			if !replicateSeen[id] {
				replicates = append(replicates, id)
				replicateSeen[id] = true
			}
		}
		sort.Strings(replicates)
		result = append(result, ReviewItem{ID: input.ID, EvidenceArea: input.EvidenceArea, Problem: strings.TrimSpace(input.Problem), Requirement: strings.TrimSpace(input.Requirement), ReplicateIDs: replicates, ReturnedVersion: a.Assessment.Version, BlockingRuleCodes: append([]string(nil), blockingRules...)})
	}
	return NewEvent(EventReviewReturned, at, ReviewData{Review: Review{ReviewedBy: strings.TrimSpace(reviewer), Reason: strings.TrimSpace(reason), Approved: false, ReviewedAt: at.UTC(), Items: result}})
}

func (a *Aggregate) ApproveReview(reviewer, reason string, at time.Time) (Event, error) {
	if a.Assessment.Status != StatusPendingReview {
		return Event{}, State("pending_review", a.Assessment.Status)
	}
	if strings.TrimSpace(reviewer) == "" {
		return Event{}, Invalid("复核人不能为空", nil)
	}
	if a.Metrics == nil {
		return Event{}, Invalid("缺少资格计算结果", nil)
	}
	if !a.MetricsCurrent() {
		return Event{}, &DomainError{Code: CodeMetricsStale, Message: "当前资格指标版本已过期，请重新计算", CurrentVersion: a.Assessment.Version}
	}
	if fields := a.unresolvedReviewItemFields(); len(fields) > 0 {
		return Event{}, &DomainError{Code: CodeReviewItemsOpen, Message: "仍有未解决的复核退回事项", Fields: fields}
	}
	if a.HasOpenDeviation() {
		return Event{}, &DomainError{Code: CodeDeviationOpen, Message: "仍有未闭环异常，不能批准", Fields: openDeviationFields(a)}
	}
	for _, hit := range a.Metrics.RuleHits {
		if hit.Blocking && !hit.Passed {
			return Event{}, &DomainError{Code: CodeRuleBlocked, Message: "规则阻断项未清零，不能批准", Fields: map[string]string{"rules." + hit.Code: hit.Message}}
		}
	}
	return NewEvent(EventReviewApproved, at, ReviewData{Review: Review{ReviewedBy: strings.TrimSpace(reviewer), Reason: strings.TrimSpace(reason), Approved: true, ReviewedAt: at.UTC()}})
}

func (a *Aggregate) ResolveReviewItem(itemID, statement string, at time.Time) (Event, error) {
	if a.Assessment.Status == StatusSealed {
		return Event{}, &DomainError{Code: CodeAlreadySealed, Message: "已封存评定不能改写退回事项"}
	}
	if a.Assessment.Status != StatusReturned {
		return Event{}, State("returned", a.Assessment.Status)
	}
	item, ok := a.ReviewItems[itemID]
	if !ok {
		return Event{}, &DomainError{Code: CodeNotFound, Message: "复核退回事项不存在"}
	}
	if item.Resolved {
		return Event{}, Invalid("复核退回事项已经解决", map[string]string{"itemId": itemID})
	}
	if strings.TrimSpace(statement) == "" {
		return Event{}, Invalid("完成说明不能为空", map[string]string{"completionStatement": "不能为空"})
	}
	if !a.reviewItemEvidenceChanged(item) {
		return Event{}, Invalid("关联证据尚无高于退回版本的有效变化", map[string]string{"itemId": itemID, "evidenceArea": string(item.EvidenceArea)})
	}
	if item.EvidenceArea != EvidenceProtocol && (a.Metrics == nil || !a.MetricsCurrent()) {
		return Event{}, &DomainError{Code: CodeMetricsStale, Message: "关联证据变化后尚未基于当前版本重新计算资格指标", CurrentVersion: a.Assessment.Version, Fields: map[string]string{"itemId": itemID}}
	}
	return NewEvent(EventReviewItemResolved, at, ReviewItemResolvedData{ItemID: itemID, CompletionStatement: strings.TrimSpace(statement), EvidenceVersion: a.Assessment.Version, ResolvedAt: at.UTC()})
}

func (a *Aggregate) ReviewItemReadiness(item ReviewItem) (bool, string) {
	if item.Resolved {
		return false, "事项已经解决"
	}
	if !a.reviewItemEvidenceChanged(item) {
		return false, "关联证据尚无高于退回版本的有效变化"
	}
	if item.EvidenceArea != EvidenceProtocol && (a.Metrics == nil || !a.MetricsCurrent()) {
		return false, "关联证据变化后尚未重新计算资格指标"
	}
	return true, "关联证据已变化且当前指标有效"
}

func (a *Aggregate) ResubmitReview(submittedBy string, at time.Time) (Event, error) {
	if a.Assessment.Status != StatusReturned {
		return Event{}, State("returned", a.Assessment.Status)
	}
	if strings.TrimSpace(submittedBy) == "" {
		return Event{}, Invalid("再次送审人不能为空", map[string]string{"submittedBy": "不能为空"})
	}
	if fields := a.unresolvedReviewItemFields(); len(fields) > 0 {
		return Event{}, &DomainError{Code: CodeReviewItemsOpen, Message: "退回事项未全部解决，不能再次送审", Fields: fields}
	}
	if a.Metrics == nil || !a.MetricsCurrent() {
		return Event{}, &DomainError{Code: CodeMetricsStale, Message: "当前资格指标版本已过期，请重新计算", CurrentVersion: a.Assessment.Version}
	}
	if a.HasOpenDeviation() {
		return Event{}, &DomainError{Code: CodeDeviationOpen, Message: "仍有未闭环异常，不能再次送审", Fields: openDeviationFields(a)}
	}
	for _, hit := range a.Metrics.RuleHits {
		if hit.Blocking && !hit.Passed {
			return Event{}, &DomainError{Code: CodeRuleBlocked, Message: "规则阻断项未清零，不能再次送审", Fields: map[string]string{"rules." + hit.Code: hit.Message}}
		}
	}
	return NewEvent(EventReviewResubmitted, at, ReviewResubmittedData{SubmittedBy: strings.TrimSpace(submittedBy), SubmittedAt: at.UTC()})
}

func (a *Aggregate) unresolvedReviewItemFields() map[string]string {
	fields := map[string]string{}
	for id, item := range a.ReviewItems {
		if !item.Resolved {
			fields["reviewItems."+id] = string(item.EvidenceArea) + ": " + item.Problem
		}
	}
	return fields
}

func openDeviationFields(a *Aggregate) map[string]string {
	fields := map[string]string{}
	for id, deviation := range a.Deviations {
		if deviation.Status == DeviationOpen {
			fields["deviations."+id] = "尚未闭环"
		}
	}
	return fields
}

func (a *Aggregate) reviewItemEvidenceChanged(item ReviewItem) bool {
	allowed := map[ReviewEvidenceArea]map[string]bool{
		EvidenceProtocol:    {EventProtocolFrozen: true},
		EvidenceObservation: {EventObservationRecorded: true, EventObservationBatchRecorded: true},
		EvidenceDeviation:   {EventDeviationRegistered: true, EventDeviationClosed: true},
		EvidenceRules:       {EventMetricsCalculated: true},
	}
	wanted := map[string]bool{}
	for _, id := range item.ReplicateIDs {
		wanted[id] = true
	}
	for _, entry := range a.Audit {
		if entry.Version <= item.ReturnedVersion || !allowed[item.EvidenceArea][entry.Type] {
			continue
		}
		// 指标与方案为整体证据，无需按重复组限定；事项未限定重复组时也不必逐组比对。
		if entry.Type == EventMetricsCalculated || len(item.ReplicateIDs) == 0 {
			return true
		}
		if a.eventTouchesReplicate(entry, wanted) {
			return true
		}
	}
	return false
}

// eventTouchesReplicate reports whether the given audit entry genuinely relates
// to any of the target replicate IDs. It decodes the structured event payload
// rather than performing raw substring matching, so an unrelated observation
// whose recordedBy or other free-text field merely contains the same identifier
// cannot be mistaken for real evidence tied to the target replicate.
func (a *Aggregate) eventTouchesReplicate(entry AuditEntry, wanted map[string]bool) bool {
	switch entry.Type {
	case EventObservationRecorded:
		var data ObservationRecordedData
		if err := json.Unmarshal(entry.Data, &data); err != nil {
			return false
		}
		return wanted[data.Observation.ReplicateID]
	case EventObservationBatchRecorded:
		var data ObservationBatchRecordedData
		if err := json.Unmarshal(entry.Data, &data); err != nil {
			return false
		}
		for _, observation := range data.Observations {
			if wanted[observation.ReplicateID] {
				return true
			}
		}
		return false
	case EventDeviationRegistered:
		var data DeviationRegisteredData
		if err := json.Unmarshal(entry.Data, &data); err != nil {
			return false
		}
		return deviationTouchesReplicates(data.Deviation, wanted)
	case EventDeviationClosed:
		var closed DeviationClosedData
		if err := json.Unmarshal(entry.Data, &closed); err != nil {
			return false
		}
		// 闭环事件本身只携带异常编号，受影响与复测重复组需要从当前异常快照取回。
		if deviation, ok := a.Deviations[closed.ID]; ok {
			return deviationTouchesReplicates(deviation, wanted)
		}
		return false
	default:
		return false
	}
}

func deviationTouchesReplicates(deviation Deviation, wanted map[string]bool) bool {
	for _, id := range deviation.AffectedReplicateIDs {
		if wanted[id] {
			return true
		}
	}
	for _, id := range deviation.RetestReplicateIDs {
		if wanted[id] {
			return true
		}
	}
	return false
}

func (a *Aggregate) SealCertificate(number, eventChainDigest string, at time.Time) (Event, error) {
	if a.Certificate != nil || a.Assessment.Status == StatusSealed {
		return Event{}, &DomainError{Code: CodeAlreadySealed, Message: "资格凭据已经封存，不能重复生成"}
	}
	if a.Assessment.Status != StatusApproved {
		return Event{}, State("approved", a.Assessment.Status)
	}
	if a.Metrics == nil || a.Protocol == nil || len(a.Reviews) == 0 {
		return Event{}, Invalid("凭据材料不完整", nil)
	}
	review := a.Reviews[len(a.Reviews)-1]
	inputDigest, err := CertificateInputDigest(a)
	if err != nil {
		return Event{}, err
	}
	certificate := QualificationCertificate{CertificateNo: number, AssessmentID: a.Assessment.ID, ProtocolDigest: a.Protocol.ContentDigest, InputDigest: inputDigest, FinalGerminationRate: a.Metrics.FinalGerminationRate, Dispersion: a.Metrics.Dispersion, ThresholdDay: a.Metrics.ThresholdDay, Decision: a.Metrics.Decision, ReviewedBy: review.ReviewedBy, ApprovedAt: review.ReviewedAt, EventChainDigest: eventChainDigest}
	certificate.CertificateDigest, err = CertificateDigest(certificate)
	if err != nil {
		return Event{}, err
	}
	return NewEvent(EventCertificateSealed, at, CertificateSealedData{Certificate: certificate})
}

func ProtocolDigest(snapshot ProtocolSnapshot) (string, error) {
	snapshot.AssessmentID = ""
	snapshot.SnapshotNo = 0
	snapshot.FrozenAt = time.Time{}
	snapshot.ContentDigest = ""
	return Digest(snapshot)
}

func CertificateInputDigest(a *Aggregate) (string, error) {
	return Digest(struct {
		Assessment   Assessment               `json:"assessment"`
		Protocol     *ProtocolSnapshot        `json:"protocol"`
		Replicates   []Replicate              `json:"replicates"`
		Observations map[string][]Observation `json:"observations"`
		Deviations   []Deviation              `json:"deviations"`
	}{a.Assessment, a.Protocol, a.SortedReplicates(), a.Observations, a.SortedDeviations()})
}

func Digest(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func CertificateDigest(value QualificationCertificate) (string, error) {
	value.CertificateDigest = ""
	return Digest(value)
}

func VerifyCertificate(value QualificationCertificate) bool {
	digest, err := CertificateDigest(value)
	return err == nil && digest == value.CertificateDigest
}
