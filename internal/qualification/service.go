package qualification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"seed-vigor-gate/internal/domain"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/store"
	"sort"
	"strings"
)

type mutation func(*domain.Aggregate) (domain.Event, string, error)

func (s *Service) Create(ctx context.Context, key string, command CreateCommand) (store.Receipt, error) {
	if strings.TrimSpace(key) == "" {
		return store.Receipt{}, domain.Invalid("Idempotency-Key 不能为空", nil)
	}
	payloadDigest := commandDigest(command)
	if old, ok := s.repository.LookupReceipt(ctx, key); ok {
		if strings.TrimSpace(command.ID) != "" && old.AssessmentID != command.ID {
			return store.Receipt{}, domain.IdempotencyConflict(key, old.EventType, domain.EventAssessmentCreated)
		}
		if replay, err := replayReceipt(old, domain.EventAssessmentCreated, payloadDigest); err != nil {
			return store.Receipt{}, err
		} else {
			return replay, nil
		}
	}
	if strings.TrimSpace(command.ID) == "" {
		command.ID = newID("asg")
	}
	event, err := domain.CreateAssessment(domain.CreateAssessmentInput{ID: command.ID, LotCode: command.LotCode, SpeciesName: command.SpeciesName, HarvestYear: command.HarvestYear, SubmittedQuantity: command.SubmittedQuantity, PretreatmentBoundary: command.PretreatmentBoundary}, s.clock.Now())
	if err != nil {
		return store.Receipt{}, err
	}
	return s.repository.AppendCommand(ctx, command.ID, 0, key, payloadDigest, event, "")
}

func (s *Service) FreezeProtocol(ctx context.Context, id, key string, command FreezeProtocolCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventProtocolFrozen, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		snapshot, issues, err := s.engine.PrepareSnapshot(command.Snapshot(), a.Assessment.SubmittedQuantity)
		if err != nil {
			return domain.Event{}, "", err
		}
		if len(issues) > 0 {
			return domain.Event{}, "", issuesError(issues)
		}
		event, err := a.FreezeProtocol(snapshot, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) PlaceReplicates(ctx context.Context, id, key string, command PlaceReplicatesCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventReplicatesPlaced, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		values := make([]domain.Replicate, 0, len(command.Replicates))
		for _, input := range command.Replicates {
			values = append(values, domain.Replicate{ID: input.ID, Label: input.Label, SownQuantity: input.SownQuantity, StartedAt: input.StartedAt})
		}
		event, err := a.PlaceReplicates(values, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) Start(ctx context.Context, id, key string, command StartCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventObservationStarted, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		event, err := a.StartObservation(s.clock.Now())
		return event, "", err
	})
}

func (s *Service) RecordObservation(ctx context.Context, id, key string, command RecordObservationCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventObservationRecorded, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		observationID := command.ID
		if observationID == "" {
			observationID = newID("obs")
		}
		event, err := a.RecordObservation(domain.ObservationInput{ID: observationID, ReplicateID: command.ReplicateID, DayNo: command.DayNo, NormalGerminated: command.NormalGerminated, AbnormalSeedlings: command.AbnormalSeedlings, HardSeeds: command.HardSeeds, DeadSeeds: command.DeadSeeds, UngerminatedSeeds: command.UngerminatedSeeds, RecordedBy: command.RecordedBy}, s.clock.Now())
		return event, "", err
	})
}

type BatchValidationResult struct {
	Valid           bool                           `json:"valid"`
	DayNo           int                            `json:"dayNo"`
	GroupCount      int                            `json:"groupCount"`
	ExpectedVersion int64                          `json:"expectedVersion"`
	CurrentVersion  int64                          `json:"currentVersion"`
	Issues          []domain.ObservationBatchIssue `json:"issues"`
}

func (s *Service) ValidateObservationBatch(ctx context.Context, id string, command RecordObservationBatchCommand) (BatchValidationResult, error) {
	a, err := s.repository.Load(ctx, id)
	if err != nil {
		return BatchValidationResult{}, err
	}
	if a.Assessment.Version != command.ExpectedVersion {
		return BatchValidationResult{}, domain.VersionConflict(a.Assessment.Version)
	}
	input, requiredIssues := batchDomainInput(command, false)
	issues := mergeBatchIssues(requiredIssues, a.ValidateObservationBatch(input))
	return BatchValidationResult{Valid: len(issues) == 0, DayNo: command.DayNo, GroupCount: len(command.Observations), ExpectedVersion: command.ExpectedVersion, CurrentVersion: a.Assessment.Version, Issues: issues}, nil
}

func (s *Service) RecordObservationBatch(ctx context.Context, id, key string, command RecordObservationBatchCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventObservationBatchRecorded, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		input, issues := batchDomainInput(command, true)
		issues = mergeBatchIssues(issues, a.ValidateObservationBatch(input))
		if len(issues) > 0 {
			fields := map[string]string{}
			for _, issue := range issues {
				prefix := "batch"
				if issue.ReplicateID != "" {
					prefix = "replicates." + issue.ReplicateID
				}
				fields[prefix+"."+issue.Field] = issue.Message
			}
			return domain.Event{}, "", domain.Invalid("同日批量观测校验未通过，整批未保存", fields)
		}
		event, _, err := a.RecordObservationBatch(input, s.clock.Now())
		return event, "", err
	})
}

func mergeBatchIssues(required, validation []domain.ObservationBatchIssue) []domain.ObservationBatchIssue {
	missingCounts := map[string]bool{}
	for _, issue := range required {
		missingCounts[issue.ReplicateID] = true
	}
	issues := append([]domain.ObservationBatchIssue(nil), required...)
	for _, issue := range validation {
		if missingCounts[issue.ReplicateID] && issue.Field == "counts" {
			continue
		}
		issues = append(issues, issue)
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].ReplicateID == issues[j].ReplicateID {
			if issues[i].Field == issues[j].Field {
				return issues[i].Code < issues[j].Code
			}
			return issues[i].Field < issues[j].Field
		}
		return issues[i].ReplicateID < issues[j].ReplicateID
	})
	return issues
}

func batchDomainInput(command RecordObservationBatchCommand, generateIDs bool) (domain.ObservationBatchInput, []domain.ObservationBatchIssue) {
	input := domain.ObservationBatchInput{DayNo: command.DayNo, RecordedBy: command.RecordedBy, Observations: make([]domain.ObservationInput, 0, len(command.Observations))}
	issues := make([]domain.ObservationBatchIssue, 0)
	for _, item := range command.Observations {
		values := []*int{item.NormalGerminated, item.AbnormalSeedlings, item.HardSeeds, item.DeadSeeds, item.UngerminatedSeeds}
		fields := []string{"normalGerminated", "abnormalSeedlings", "hardSeeds", "deadSeeds", "ungerminatedSeeds"}
		missing := false
		for index, value := range values {
			if value == nil {
				missing = true
				issues = append(issues, domain.ObservationBatchIssue{ReplicateID: item.ReplicateID, Field: fields[index], Code: "REQUIRED", Message: "必填计数不能遗漏"})
			}
		}
		id := item.ID
		if generateIDs && id == "" {
			id = newID("obs")
		}
		observation := domain.ObservationInput{ID: id, ReplicateID: item.ReplicateID}
		if !missing {
			observation.NormalGerminated = *item.NormalGerminated
			observation.AbnormalSeedlings = *item.AbnormalSeedlings
			observation.HardSeeds = *item.HardSeeds
			observation.DeadSeeds = *item.DeadSeeds
			observation.UngerminatedSeeds = *item.UngerminatedSeeds
		}
		input.Observations = append(input.Observations, observation)
	}
	return input, issues
}

func (s *Service) RegisterDeviation(ctx context.Context, id, key string, command RegisterDeviationCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventDeviationRegistered, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		deviationID := command.ID
		if deviationID == "" {
			deviationID = newID("dev")
		}
		event, err := a.RegisterDeviation(domain.RegisterDeviationInput{ID: deviationID, Category: command.Category, OccurredAt: command.OccurredAt, AffectedReplicateIDs: command.AffectedReplicateIDs, Description: command.Description, Disposition: command.Disposition}, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) CloseDeviation(ctx context.Context, id, key string, command CloseDeviationCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventDeviationClosed, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		event, err := a.CloseDeviation(command.DeviationID, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) Calculate(ctx context.Context, id, key string, command CalculateCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventMetricsCalculated, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		if a.Assessment.Status != domain.StatusObserving && a.Assessment.Status != domain.StatusReturned {
			return domain.Event{}, "", domain.State("observing 或 returned", a.Assessment.Status)
		}
		metrics, err := s.engine.Calculate(a, s.clock.Now())
		if err != nil {
			return domain.Event{}, "", err
		}
		event, err := domain.NewEvent(domain.EventMetricsCalculated, s.clock.Now(), domain.MetricsCalculatedData{Metrics: metrics})
		return event, "", err
	})
}

func (s *Service) ReturnReview(ctx context.Context, id, key string, value any) (store.Receipt, error) {
	command, ok := value.(ReturnReviewCommand)
	if !ok {
		if legacy, legacyOK := value.(ReviewCommand); legacyOK {
			command = ReturnReviewCommand{Versioned: legacy.Versioned, Reviewer: legacy.Reviewer, Reason: legacy.Reason}
		} else {
			return store.Receipt{}, domain.Invalid("复核退回命令类型无效", nil)
		}
	}
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventReviewReturned, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		items := make([]domain.ReviewItemInput, 0, len(command.Items))
		for _, item := range command.Items {
			itemID := item.ID
			if itemID == "" {
				itemID = newID("revitem")
			}
			items = append(items, domain.ReviewItemInput{ID: itemID, EvidenceArea: domain.ReviewEvidenceArea(item.EvidenceArea), Problem: item.Problem, Requirement: item.Requirement, ReplicateIDs: item.ReplicateIDs})
		}
		event, err := a.ReturnReviewWithItems(command.Reviewer, command.Reason, items, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) ResolveReviewItem(ctx context.Context, id, key string, command ResolveReviewItemCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventReviewItemResolved, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		event, err := a.ResolveReviewItem(command.ItemID, command.CompletionStatement, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) ResubmitReview(ctx context.Context, id, key string, command ResubmitReviewCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventReviewResubmitted, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		event, err := a.ResubmitReview(command.SubmittedBy, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) ApproveReview(ctx context.Context, id, key string, command ReviewCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventReviewApproved, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		event, err := a.ApproveReview(command.Reviewer, command.Reason, s.clock.Now())
		return event, "", err
	})
}

func (s *Service) Seal(ctx context.Context, id, key string, command SealCommand) (store.Receipt, error) {
	return s.change(ctx, id, key, command.ExpectedVersion, domain.EventCertificateSealed, commandDigest(command), func(a *domain.Aggregate) (domain.Event, string, error) {
		chain, err := s.repository.ChainDigest(ctx, id)
		if err != nil {
			return domain.Event{}, "", err
		}
		number := certificateNumber(a)
		event, err := a.SealCertificate(number, chain, s.clock.Now())
		return event, number, err
	})
}

func (s *Service) change(ctx context.Context, id, key string, expected int64, eventType, payloadDigest string, fn mutation) (store.Receipt, error) {
	if strings.TrimSpace(key) == "" {
		return store.Receipt{}, domain.Invalid("Idempotency-Key 不能为空", nil)
	}
	if old, ok := s.repository.LookupReceipt(ctx, key); ok {
		if old.AssessmentID != id {
			return store.Receipt{}, domain.IdempotencyConflict(key, old.EventType, eventType)
		}
		if replay, err := replayReceipt(old, eventType, payloadDigest); err != nil {
			return store.Receipt{}, err
		} else {
			return replay, nil
		}
	}
	aggregate, err := s.repository.Load(ctx, id)
	if err != nil {
		return store.Receipt{}, err
	}
	if aggregate.Assessment.Version != expected {
		return store.Receipt{}, domain.VersionConflict(aggregate.Assessment.Version)
	}
	event, certificateNo, err := fn(aggregate)
	if err != nil {
		return store.Receipt{}, err
	}
	return s.repository.AppendCommand(ctx, id, expected, key, payloadDigest, event, certificateNo)
}

func replayReceipt(old store.Receipt, eventType, payloadDigest string) (store.Receipt, error) {
	if old.EventType != eventType || old.PayloadDigest != payloadDigest {
		return store.Receipt{}, domain.IdempotencyConflict(old.IdempotencyKey, old.EventType, eventType)
	}
	return old, nil
}

func commandDigest(command any) string {
	b, err := json.Marshal(command)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func issuesError(issues []protocol.Issue) error {
	fields := make(map[string]string, len(issues))
	for _, issue := range issues {
		fields[issue.Field] = issue.Message
	}
	return domain.Invalid("试验方案存在完整性问题", fields)
}

func newID(prefix string) string {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("系统随机源不可用: %v", err))
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}

func certificateNumber(a *domain.Aggregate) string {
	id := strings.ToUpper(strings.ReplaceAll(a.Assessment.ID, "-", ""))
	if len(id) > 12 {
		id = id[:12]
	}
	return fmt.Sprintf("SVG-%s-%04d", id, a.Assessment.Version+1)
}
