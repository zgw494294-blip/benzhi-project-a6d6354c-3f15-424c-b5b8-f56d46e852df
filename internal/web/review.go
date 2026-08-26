package web

import (
	"net/http"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
)

func (h *Handler) CalculateHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.CalculateCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) { return h.service.Calculate(r.Context(), id, key, command) })
}

func (h *Handler) ReturnReviewHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.ReturnReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) { return h.service.ReturnReview(r.Context(), id, key, command) })
}

func (h *Handler) ResolveReviewItemHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.ResolveReviewItemCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.ResolveReviewItem(r.Context(), id, key, command)
	})
}

func (h *Handler) ResubmitReviewHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.ResubmitReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) {
		return h.service.ResubmitReview(r.Context(), id, key, command)
	})
}

func (h *Handler) ApproveReviewHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.ReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusOK, func(key string) (store.Receipt, error) { return h.service.ApproveReview(r.Context(), id, key, command) })
}

func (h *Handler) SealCertificateHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var command qualification.SealCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeInputError(w, err)
		return
	}
	h.finishCommand(w, r, id, http.StatusCreated, func(key string) (store.Receipt, error) { return h.service.Seal(r.Context(), id, key, command) })
}
