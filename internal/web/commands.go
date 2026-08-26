package web

import (
	"context"
	"net/http"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
)

type commandResponse struct {
	Receipt    store.Receipt                `json:"receipt"`
	Assessment qualification.AssessmentView `json:"assessment"`
}

func (h *Handler) finishCommand(w http.ResponseWriter, r *http.Request, id string, status int, invoke func(string) (store.Receipt, error)) {
	key, err := idempotencyKey(r)
	if err != nil {
		writeInputError(w, err)
		return
	}
	receipt, err := invoke(key)
	if err != nil {
		writeError(w, err)
		return
	}
	view, err := h.service.Get(r.Context(), receipt.AssessmentID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, commandResponse{Receipt: receipt, Assessment: view})
}

func (h *Handler) CreateAssessmentHandler(w http.ResponseWriter, r *http.Request) {
	var command qualification.CreateCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, command.ID, http.StatusCreated, func(key string) (store.Receipt, error) {
		return h.service.Create(context.WithoutCancel(r.Context()), key, command)
	})
}

func (h *Handler) FreezeProtocolHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.FreezeProtocolCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.FreezeProtocol(context.WithoutCancel(r.Context()), id, key, command)
	})
}

func (h *Handler) PlaceReplicatesHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.PlaceReplicatesCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.PlaceReplicates(context.WithoutCancel(r.Context()), id, key, command)
	})
}

func (h *Handler) StartObservationHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.StartCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.Start(context.WithoutCancel(r.Context()), id, key, command)
	})
}

func (h *Handler) RecordObservationHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var request struct {
		qualification.Versioned
		ID                string                                `json:"id"`
		ReplicateID       string                                `json:"replicateId"`
		DayNo             int                                   `json:"dayNo"`
		NormalGerminated  int                                   `json:"normalGerminated"`
		AbnormalSeedlings int                                   `json:"abnormalSeedlings"`
		HardSeeds         int                                   `json:"hardSeeds"`
		DeadSeeds         int                                   `json:"deadSeeds"`
		UngerminatedSeeds int                                   `json:"ungerminatedSeeds"`
		RecordedBy        string                                `json:"recordedBy"`
		Observations      []qualification.BatchObservationInput `json:"observations"`
		ValidateOnly      bool                                  `json:"validateOnly,omitempty"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeInputError(w, err)
		return
	}
	if request.Observations != nil {
		command := qualification.RecordObservationBatchCommand{Versioned: request.Versioned, DayNo: request.DayNo, RecordedBy: request.RecordedBy, Observations: request.Observations, ValidateOnly: request.ValidateOnly}
		if request.ValidateOnly {
			result, err := h.service.ValidateObservationBatch(r.Context(), id, command)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
			return h.service.RecordObservationBatch(r.Context(), id, key, command)
		})
		return
	}
	command := qualification.RecordObservationCommand{Versioned: request.Versioned, ID: request.ID, ReplicateID: request.ReplicateID, DayNo: request.DayNo, NormalGerminated: request.NormalGerminated, AbnormalSeedlings: request.AbnormalSeedlings, HardSeeds: request.HardSeeds, DeadSeeds: request.DeadSeeds, UngerminatedSeeds: request.UngerminatedSeeds, RecordedBy: request.RecordedBy}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.RecordObservation(context.WithoutCancel(r.Context()), id, key, command)
	})
}

func (h *Handler) RegisterDeviationHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.RegisterDeviationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.RegisterDeviation(context.WithoutCancel(r.Context()), id, key, command)
	})
}

func (h *Handler) CloseDeviationHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.CloseDeviationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.CloseDeviation(r.Context(), id, key, command)
	})
}
