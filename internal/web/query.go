package web

import "net/http"

func (h *Handler) GetAssessmentHandler(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) VerifyCertificateHandler(w http.ResponseWriter, r *http.Request) {
	verification, err := h.service.VerifyCertificate(r.Context(), r.PathValue("number"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, verification)
}
