package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"seed-vigor-gate/internal/domain"
)

func (l *Ledger) recover() error {
	file, err := os.Open(l.ledgerPath)
	if errors.Is(err, os.ErrNotExist) {
		return l.checkSnapshot(nil)
	}
	if err != nil {
		return fmt.Errorf("读取事件账本: %w", err)
	}
	defer file.Close()
	lineNo := 0
	err = scanLines(file, func(line []byte) error {
		lineNo++
		var record LedgerRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return fmt.Errorf("第 %d 行 JSON 无效: %w", lineNo, err)
		}
		if record.SchemaVersion != SchemaVersion {
			return fmt.Errorf("第 %d 行 schemaVersion %d 未知", lineNo, record.SchemaVersion)
		}
		aggregate, exists := l.aggregates[record.AssessmentID]
		expectedSequence := int64(1)
		if exists {
			expectedSequence = aggregate.Assessment.Version + 1
		}
		if record.Sequence != expectedSequence {
			return fmt.Errorf("评定 %s 事件序号断裂: 得到 %d，需要 %d", record.AssessmentID, record.Sequence, expectedSequence)
		}
		if record.PreviousDigest != l.chains[record.AssessmentID] {
			return fmt.Errorf("评定 %s 前序摘要不匹配", record.AssessmentID)
		}
		digest, err := recordDigest(record)
		if err != nil {
			return err
		}
		if digest != record.Digest {
			return fmt.Errorf("第 %d 行摘要不匹配", lineNo)
		}
		if old, duplicate := l.receipts[record.IdempotencyKey]; duplicate && (old.AssessmentID != record.AssessmentID || old.Version != record.Sequence || old.PayloadDigest != record.PayloadDigest) {
			return fmt.Errorf("幂等键 %s 被不同事件复用", record.IdempotencyKey)
		}
		if !exists {
			aggregate = domain.EmptyAggregate()
		} else {
			aggregate, err = aggregate.Clone()
			if err != nil {
				return err
			}
		}
		if err := aggregate.Apply(record.Event); err != nil {
			return fmt.Errorf("重放第 %d 行: %w", lineNo, err)
		}
		if aggregate.Assessment.ID != record.AssessmentID {
			return fmt.Errorf("第 %d 行聚合编号不一致", lineNo)
		}
		if record.Event.Type == domain.EventCertificateSealed {
			if aggregate.Certificate == nil || !domain.VerifyCertificate(*aggregate.Certificate) {
				return fmt.Errorf("评定 %s 的资格凭据摘要无效", record.AssessmentID)
			}
			if aggregate.Certificate.EventChainDigest != record.PreviousDigest {
				return fmt.Errorf("评定 %s 的凭据事件链摘要无效", record.AssessmentID)
			}
			if record.CertificateNo != aggregate.Certificate.CertificateNo {
				return fmt.Errorf("评定 %s 的凭据编号索引不一致", record.AssessmentID)
			}
			l.certificates[record.CertificateNo] = record.AssessmentID
		}
		l.aggregates[record.AssessmentID] = aggregate
		l.chains[record.AssessmentID] = record.Digest
		l.receipts[record.IdempotencyKey] = receiptFor(record, aggregate)
		l.records[record.AssessmentID] = append(l.records[record.AssessmentID], record)
		return nil
	})
	if err != nil {
		return fmt.Errorf("事件账本完整性校验失败: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return l.checkSnapshot(info)
}
