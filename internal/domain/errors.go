package domain

import "fmt"

type ErrorCode string

const (
	CodeInvalid           ErrorCode = "INVALID_INPUT"
	CodeConflict          ErrorCode = "VERSION_CONFLICT"
	CodeState             ErrorCode = "STATE_CONFLICT"
	CodeNotFound          ErrorCode = "NOT_FOUND"
	CodeIntegrity         ErrorCode = "INTEGRITY_ERROR"
	CodeAlreadySealed     ErrorCode = "ALREADY_SEALED"
	CodeReviewItemsOpen   ErrorCode = "REVIEW_ITEMS_OPEN"
	CodeMetricsStale      ErrorCode = "METRICS_STALE"
	CodeDeviationOpen     ErrorCode = "DEVIATION_OPEN"
	CodeRuleBlocked       ErrorCode = "RULE_BLOCKED"
	CodeIdempotencyConflict ErrorCode = "IDEMPOTENCY_CONFLICT"
)

type DomainError struct {
	Code           ErrorCode         `json:"code"`
	Message        string            `json:"message"`
	Fields         map[string]string `json:"fields,omitempty"`
	CurrentVersion int64             `json:"currentVersion,omitempty"`
}

func (e *DomainError) Error() string { return e.Message }

func Invalid(message string, fields map[string]string) error {
	return &DomainError{Code: CodeInvalid, Message: message, Fields: fields}
}

func State(expected string, actual Status) error {
	return &DomainError{Code: CodeState, Message: fmt.Sprintf("当前状态 %s 不允许此操作，需要 %s", actual, expected)}
}

func VersionConflict(current int64) error {
	return &DomainError{Code: CodeConflict, Message: "评定版本已变化，请刷新后重试", CurrentVersion: current}
}

func IdempotencyConflict(key, stored, requested string) error {
	return &DomainError{Code: CodeIdempotencyConflict, Message: "Idempotency-Key 已用于不同命令，不能重放其他命令的结果", Fields: map[string]string{"idempotencyKey": key, "storedEventType": stored, "requestedEventType": requested}}
}
