package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func (l *Ledger) writeSnapshotLocked() error {
	info, err := l.file.Stat()
	if err != nil {
		return fmt.Errorf("读取账本大小: %w", err)
	}
	versions := make(map[string]int64, len(l.aggregates))
	for id, aggregate := range l.aggregates {
		versions[id] = aggregate.Assessment.Version
	}
	value := snapshotFile{SchemaVersion: SchemaVersion, LedgerSize: info.Size(), Chains: copyStrings(l.chains), Versions: versions, Certificates: copyStrings(l.certificates)}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(l.dir, ".snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时快照: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() { _ = temp.Close(); _ = os.Remove(tempName) }
	if err := temp.Chmod(0o640); err != nil {
		cleanup()
		return err
	}
	if _, err := temp.Write(b); err != nil {
		cleanup()
		return fmt.Errorf("写入快照: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("同步快照: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	if err := os.Rename(tempName, l.snapshotPath); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("原子替换快照: %w", err)
	}
	dir, err := os.Open(l.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (l *Ledger) checkSnapshot(ledgerInfo os.FileInfo) error {
	b, err := os.ReadFile(l.snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var value snapshotFile
	if err := json.Unmarshal(b, &value); err != nil {
		return fmt.Errorf("快照 JSON 无效: %w", err)
	}
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("快照 schemaVersion %d 未知", value.SchemaVersion)
	}
	if ledgerInfo == nil {
		if value.LedgerSize != 0 || len(value.Versions) != 0 {
			return errors.New("快照存在但事件账本缺失")
		}
		return nil
	}
	if value.LedgerSize > ledgerInfo.Size() {
		return errors.New("快照记录的账本位置超出实际账本")
	}
	if value.LedgerSize < ledgerInfo.Size() {
		return nil
	}
	for id, aggregate := range l.aggregates {
		if value.Versions[id] != aggregate.Assessment.Version || value.Chains[id] != l.chains[id] {
			return fmt.Errorf("评定 %s 快照投影不一致", id)
		}
	}
	for number, id := range l.certificates {
		if value.Certificates[number] != id {
			return fmt.Errorf("凭据 %s 快照索引不一致", number)
		}
	}
	return nil
}

func copyStrings(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
