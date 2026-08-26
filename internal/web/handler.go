package web

import (
	"net/http"
	"seed-vigor-gate/internal/qualification"
)

type Handler struct {
	service *qualification.Service
	mux     *http.ServeMux
}

func NewHandler(service *qualification.Service) *Handler {
	h := &Handler{service: service, mux: http.NewServeMux()}
	h.routes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	h.mux.HandleFunc("GET /", h.WorkbenchHandler)
	h.mux.HandleFunc("GET /assets/app.css", h.StylesHandler)
	h.mux.HandleFunc("GET /assets/app.js", h.ScriptHandler)
	h.mux.HandleFunc("POST /api/assessments", h.CreateAssessmentHandler)
	h.mux.HandleFunc("GET /api/assessments/{id}", h.GetAssessmentHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/protocol/freeze", h.FreezeProtocolHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/replicates", h.PlaceReplicatesHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/start", h.StartObservationHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/observations", h.RecordObservationHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/deviations", h.RegisterDeviationHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/deviations/close", h.CloseDeviationHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/calculate", h.CalculateHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/review/return", h.ReturnReviewHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/review/items/resolve", h.ResolveReviewItemHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/review/resubmit", h.ResubmitReviewHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/review/approve", h.ApproveReviewHandler)
	h.mux.HandleFunc("POST /api/assessments/{id}/seal", h.SealCertificateHandler)
	h.mux.HandleFunc("GET /api/certificates/{number}/verify", h.VerifyCertificateHandler)
}
