package qualification

import (
	"context"
	"encoding/json"
	"seed-vigor-gate/internal/domain"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/store"
	"time"
)

type AssessmentView struct {
	Assessment              domain.Assessment                `json:"assessment"`
	Protocol                *domain.ProtocolSnapshot         `json:"protocol,omitempty"`
	Replicates              []domain.Replicate               `json:"replicates"`
	Observations            map[string][]domain.Observation  `json:"observations"`
	Deviations              []domain.Deviation               `json:"deviations"`
	Metrics                 *domain.Metrics                  `json:"metrics,omitempty"`
	Reviews                 []domain.Review                  `json:"reviews"`
	Certificate             *domain.QualificationCertificate `json:"certificate,omitempty"`
	Audit                   []domain.AuditEntry              `json:"audit"`
	Checklist               []ChecklistItem                  `json:"checklist"`
	Progress                protocol.ProgressProjection      `json:"progress"`
	SampleBoundary          domain.SampleBoundary            `json:"sampleBoundary"`
	ReviewItems             []domain.ReviewItem              `json:"reviewItems"`
	CertificateVerification *CertificateVerification         `json:"certificateVerification,omitempty"`
}

type CertificateVerificationItem struct {
	Code    string `json:"code"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type CertificateVerification struct {
	Certificate domain.QualificationCertificate `json:"certificate"`
	Valid       bool                            `json:"valid"`
	Message     string                          `json:"message"`
	Algorithm   string                          `json:"algorithm"`
	CheckedAt   time.Time                       `json:"checkedAt"`
	Items       []CertificateVerificationItem   `json:"items"`
}

func (s *Service) Get(ctx context.Context, id string) (AssessmentView, error) {
	a, err := s.repository.Load(ctx, id)
	if err != nil {
		return AssessmentView{}, err
	}
	reviewItems := a.SortedReviewItems()
	for index := range reviewItems {
		reviewItems[index].CanResolve, reviewItems[index].ResolveBlocker = a.ReviewItemReadiness(reviewItems[index])
	}
	deviations := a.SortedDeviations()
	for index := range deviations {
		if deviations[index].Status == domain.DeviationOpen {
			deviations[index].ReadinessIssues = a.DeviationReadiness(deviations[index])
			deviations[index].ReadyToClose = len(deviations[index].ReadinessIssues) == 0
		}
	}
	view := AssessmentView{Assessment: a.Assessment, Protocol: a.Protocol, Replicates: a.SortedReplicates(), Observations: a.Observations, Deviations: deviations, Metrics: a.Metrics, Reviews: append([]domain.Review(nil), a.Reviews...), Certificate: a.Certificate, Audit: append([]domain.AuditEntry(nil), a.Audit...), Checklist: BuildChecklist(a), Progress: s.engine.Progress(a), SampleBoundary: a.SampleBoundary(), ReviewItems: reviewItems}
	if a.Certificate != nil {
		verification, verifyErr := s.VerifyCertificate(ctx, a.Certificate.CertificateNo)
		if verifyErr != nil {
			return AssessmentView{}, verifyErr
		}
		view.CertificateVerification = &verification
	}
	return view, nil
}

func (s *Service) VerifyCertificate(ctx context.Context, number string) (CertificateVerification, error) {
	if s.verification != nil && s.verificationCertificateNo == number {
		cached := *s.verification
		cached.Items = append([]CertificateVerificationItem(nil), s.verification.Items...)
		return cached, nil
	}
	materialRepository, ok := s.repository.(store.CertificateMaterialRepository)
	if !ok {
		return CertificateVerification{}, &domain.DomainError{Code: domain.CodeIntegrity, Message: "存储未提供凭据历史材料读取能力"}
	}
	material, err := materialRepository.LoadCertificateMaterial(ctx, number)
	if err != nil {
		return CertificateVerification{}, err
	}
	certificate := material.Certificate
	items := make([]CertificateVerificationItem, 0, 5)
	certificatePassed := domain.VerifyCertificate(certificate)
	items = append(items, verificationItem("CERTIFICATE_DIGEST", certificatePassed, "凭据内容摘要与封存值一致", "凭据内容摘要不一致"))
	protocolPassed := false
	inputPassed := false
	reviewPassed := false
	if material.PreSealAggregate != nil && material.PreSealAggregate.Protocol != nil {
		protocolDigest, digestErr := domain.ProtocolDigest(*material.PreSealAggregate.Protocol)
		protocolPassed = digestErr == nil && protocolDigest == certificate.ProtocolDigest && protocolDigest == material.PreSealAggregate.Protocol.ContentDigest
		inputDigest, inputErr := domain.CertificateInputDigest(material.PreSealAggregate)
		inputPassed = inputErr == nil && inputDigest == certificate.InputDigest
		reviewPassed = reviewMaterialMatches(certificate, material)
	}
	items = append(items, verificationItem("PROTOCOL_DIGEST", protocolPassed, "冻结方案摘要一致", "冻结方案摘要不一致"))
	items = append(items, verificationItem("INPUT_DIGEST", inputPassed, "有效输入摘要一致", "有效输入摘要不一致"))
	chainPassed := material.ChainValid && material.PreSealChainDigest == certificate.EventChainDigest
	items = append(items, verificationItem("EVENT_CHAIN", chainPassed, "封存时事件链连续且摘要一致", "封存时事件链不连续或摘要不一致"))
	items = append(items, verificationItem("REVIEW_MATERIAL", reviewPassed, "评定编号、指标、结论与复核材料一致", "评定编号、指标、结论或复核材料不一致"))
	valid := true
	for _, item := range items {
		valid = valid && item.Passed
	}
	message := "资格凭据五项一致性校验通过"
	if !valid {
		message = "资格凭据一致性校验失败，请查看失败项"
	}
	result := CertificateVerification{Certificate: certificate, Valid: valid, Message: message, Algorithm: "SHA-256/canonical-json", CheckedAt: s.clock.Now(), Items: items}
	cached := result
	cached.Items = append([]CertificateVerificationItem(nil), result.Items...)
	s.verificationCertificateNo = number
	s.verification = &cached
	return result, nil
}

func verificationItem(code string, passed bool, success, failure string) CertificateVerificationItem {
	message := failure
	if passed {
		message = success
	}
	return CertificateVerificationItem{Code: code, Passed: passed, Message: message}
}

func reviewMaterialMatches(certificate domain.QualificationCertificate, material store.CertificateMaterial) bool {
	if material.PreSealAggregate == nil || material.PreSealAggregate.Metrics == nil || len(material.PreSealAggregate.Reviews) == 0 {
		return false
	}
	var sealed domain.CertificateSealedData
	if json.Unmarshal(material.SealEvent.Data, &sealed) != nil {
		return false
	}
	review := material.PreSealAggregate.Reviews[len(material.PreSealAggregate.Reviews)-1]
	metrics := material.PreSealAggregate.Metrics
	fromEvent := sealed.Certificate
	return fromEvent.AssessmentID == certificate.AssessmentID && certificate.AssessmentID == material.PreSealAggregate.Assessment.ID &&
		fromEvent.FinalGerminationRate == certificate.FinalGerminationRate && certificate.FinalGerminationRate == metrics.FinalGerminationRate &&
		fromEvent.Dispersion == certificate.Dispersion && certificate.Dispersion == metrics.Dispersion &&
		fromEvent.ThresholdDay == certificate.ThresholdDay && certificate.ThresholdDay == metrics.ThresholdDay &&
		fromEvent.Decision == certificate.Decision && certificate.Decision == metrics.Decision &&
		fromEvent.ReviewedBy == certificate.ReviewedBy && certificate.ReviewedBy == review.ReviewedBy &&
		fromEvent.ApprovedAt.Equal(certificate.ApprovedAt) && certificate.ApprovedAt.Equal(review.ReviewedAt)
}
