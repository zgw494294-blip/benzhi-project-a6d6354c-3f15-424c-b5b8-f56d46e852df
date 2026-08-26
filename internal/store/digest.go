package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func recordDigest(record LedgerRecord) (string, error) {
	record.Digest = ""
	b, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
