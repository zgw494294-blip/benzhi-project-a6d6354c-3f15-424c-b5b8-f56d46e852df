package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"seed-vigor-gate/internal/domain"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code           string            `json:"code"`
	Message        string            `json:"message"`
	Fields         map[string]string `json:"fields,omitempty"`
	CurrentVersion int64             `json:"currentVersion,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	var de *domain.DomainError
	if errors.As(err, &de) {
		status := http.StatusUnprocessableEntity
		switch de.Code {
		case domain.CodeNotFound:
			status = http.StatusNotFound
		case domain.CodeConflict:
			status = http.StatusConflict
		case domain.CodeState, domain.CodeAlreadySealed, domain.CodeReviewItemsOpen, domain.CodeMetricsStale, domain.CodeDeviationOpen, domain.CodeRuleBlocked, domain.CodeIdempotencyConflict:
			status = http.StatusConflict
		case domain.CodeIntegrity:
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, errorEnvelope{Error: apiError{Code: string(de.Code), Message: de.Message, Fields: de.Fields, CurrentVersion: de.CurrentVersion}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{Code: "INTERNAL", Message: "服务处理失败"}})
}

func writeInputError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, errorEnvelope{Error: apiError{Code: "BAD_REQUEST", Message: err.Error()}})
}
