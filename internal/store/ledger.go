package store

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"seed-vigor-gate/internal/domain"
	"sync"
)

type Ledger struct {
	mu           sync.Mutex
	dir          string
	ledgerPath   string
	snapshotPath string
	file         *os.File
	aggregates   map[string]*domain.Aggregate
	chains       map[string]string
	receipts     map[string]Receipt
	certificates map[string]string
	records      map[string][]LedgerRecord
	closed       bool
}

func Open(dir string) (*Ledger, error) {
	if dir == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	l := &Ledger{dir: dir, ledgerPath: filepath.Join(dir, "events.jsonl"), snapshotPath: filepath.Join(dir, "snapshot.json"), aggregates: map[string]*domain.Aggregate{}, chains: map[string]string{}, receipts: map[string]Receipt{}, certificates: map[string]string{}, records: map[string][]LedgerRecord{}}
	if err := l.recover(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(l.ledgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("打开事件账本: %w", err)
	}
	l.file = file
	return l, nil
}

func (l *Ledger) Load(_ context.Context, id string) (*domain.Aggregate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	value, ok := l.aggregates[id]
	if !ok {
		return nil, &domain.DomainError{Code: domain.CodeNotFound, Message: "评定不存在"}
	}
	return value.Clone()
}

func (l *Ledger) LookupReceipt(_ context.Context, key string) (Receipt, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	value, ok := l.receipts[key]
	if ok {
		value.Replayed = true
	}
	return value, ok
}

func (l *Ledger) ChainDigest(_ context.Context, id string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.aggregates[id]; !ok {
		return "", &domain.DomainError{Code: domain.CodeNotFound, Message: "评定不存在"}
	}
	return l.chains[id], nil
}

func (l *Ledger) FindCertificate(_ context.Context, number string) (domain.QualificationCertificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id, ok := l.certificates[number]
	if !ok {
		return domain.QualificationCertificate{}, &domain.DomainError{Code: domain.CodeNotFound, Message: "资格凭据不存在"}
	}
	aggregate := l.aggregates[id]
	if aggregate == nil || aggregate.Certificate == nil {
		return domain.QualificationCertificate{}, &domain.DomainError{Code: domain.CodeIntegrity, Message: "凭据索引与聚合不一致"}
	}
	return *aggregate.Certificate, nil
}

func (l *Ledger) LoadCertificateMaterial(_ context.Context, number string) (CertificateMaterial, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id, ok := l.certificates[number]
	if !ok {
		return CertificateMaterial{}, &domain.DomainError{Code: domain.CodeNotFound, Message: "资格凭据不存在"}
	}
	current := l.aggregates[id]
	if current == nil || current.Certificate == nil {
		return CertificateMaterial{}, &domain.DomainError{Code: domain.CodeIntegrity, Message: "凭据索引与聚合不一致"}
	}
	preSeal := domain.EmptyAggregate()
	runningDigest := ""
	chainValid := true
	for _, record := range l.records[id] {
		if record.Event.Type == domain.EventCertificateSealed && record.CertificateNo == number {
			copyAggregate, err := preSeal.Clone()
			if err != nil {
				return CertificateMaterial{}, err
			}
			digest, err := recordDigest(record)
			if err != nil {
				return CertificateMaterial{}, err
			}
			chainValid = chainValid && record.PreviousDigest == runningDigest && digest == record.Digest
			return CertificateMaterial{Certificate: *current.Certificate, PreSealAggregate: copyAggregate, SealEvent: record.Event, PreSealChainDigest: runningDigest, ChainValid: chainValid}, nil
		}
		digest, err := recordDigest(record)
		if err != nil {
			return CertificateMaterial{}, err
		}
		if record.PreviousDigest != runningDigest || digest != record.Digest {
			chainValid = false
		}
		if err := preSeal.Apply(record.Event); err != nil {
			return CertificateMaterial{}, err
		}
		runningDigest = record.Digest
	}
	return CertificateMaterial{}, &domain.DomainError{Code: domain.CodeIntegrity, Message: "未找到凭据封存事件材料"}
}

func (l *Ledger) Append(_ context.Context, id string, expected int64, key string, event domain.Event, certificateNo string) (Receipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return Receipt{}, errors.New("存储已经关闭")
	}
	if key == "" {
		return Receipt{}, domain.Invalid("Idempotency-Key 不能为空", nil)
	}
	if existing, ok := l.receipts[key]; ok {
		existing.Replayed = true
		return existing, nil
	}
	current := int64(0)
	aggregate, exists := l.aggregates[id]
	if exists {
		current = aggregate.Assessment.Version
	}
	if current != expected {
		return Receipt{}, domain.VersionConflict(current)
	}
	if !exists && event.Type != domain.EventAssessmentCreated {
		return Receipt{}, &domain.DomainError{Code: domain.CodeNotFound, Message: "评定不存在"}
	}
	if exists && event.Type == domain.EventAssessmentCreated {
		return Receipt{}, domain.Invalid("评定编号已经存在", nil)
	}
	next := domain.EmptyAggregate()
	if exists {
		var err error
		next, err = aggregate.Clone()
		if err != nil {
			return Receipt{}, err
		}
	}
	if err := next.Apply(event); err != nil {
		return Receipt{}, err
	}
	record := LedgerRecord{SchemaVersion: SchemaVersion, AssessmentID: id, Sequence: current + 1, PreviousDigest: l.chains[id], IdempotencyKey: key, CertificateNo: certificateNo, Event: event}
	digest, err := recordDigest(record)
	if err != nil {
		return Receipt{}, err
	}
	record.Digest = digest
	line, err := json.Marshal(record)
	if err != nil {
		return Receipt{}, err
	}
	line = append(line, '\n')
	if _, err := l.file.Write(line); err != nil {
		return Receipt{}, fmt.Errorf("追加事件: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return Receipt{}, fmt.Errorf("同步事件账本: %w", err)
	}
	receipt := receiptFor(record, next)
	previousCertificateAssessment, hadCertificate := l.certificates[certificateNo]
	l.aggregates[id] = next
	l.chains[id] = digest
	l.receipts[key] = receipt
	l.records[id] = append(l.records[id], record)
	if certificateNo != "" {
		l.certificates[certificateNo] = id
	}
	if err := l.writeSnapshotLocked(); err != nil {
		if exists {
			l.aggregates[id] = aggregate
			l.chains[id] = record.PreviousDigest
		} else {
			delete(l.aggregates, id)
			delete(l.chains, id)
		}
		delete(l.receipts, key)
		l.records[id] = l.records[id][:len(l.records[id])-1]
		if certificateNo != "" {
			if hadCertificate {
				l.certificates[certificateNo] = previousCertificateAssessment
			} else {
				delete(l.certificates, certificateNo)
			}
		}
		return Receipt{}, err
	}
	return receipt, nil
}

func receiptFor(record LedgerRecord, aggregate *domain.Aggregate) Receipt {
	receipt := Receipt{IdempotencyKey: record.IdempotencyKey, AssessmentID: record.AssessmentID, Version: record.Sequence, EventType: record.Event.Type, CertificateNo: record.CertificateNo}
	if record.Event.Type == domain.EventObservationBatchRecorded {
		var data domain.ObservationBatchRecordedData
		if json.Unmarshal(record.Event.Data, &data) == nil {
			receipt.SuccessfulGroups = len(data.Observations)
		}
		ready := aggregate.Protocol != nil
		for id, replicate := range aggregate.Replicates {
			if replicate.Status != domain.ReplicateVoid && !aggregate.HasTerminalObservation(id) {
				ready = false
				break
			}
		}
		receipt.TerminalReady = ready
	}
	return receipt
}

func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	if l.file == nil {
		return nil
	}
	if err := l.file.Sync(); err != nil {
		_ = l.file.Close()
		return err
	}
	return l.file.Close()
}

func scanLines(file *os.File, handle func([]byte) error) error {
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if line[len(line)-1] != '\n' {
				return errors.New("事件账本末行不完整")
			}
			if err2 := handle(line[:len(line)-1]); err2 != nil {
				return err2
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
