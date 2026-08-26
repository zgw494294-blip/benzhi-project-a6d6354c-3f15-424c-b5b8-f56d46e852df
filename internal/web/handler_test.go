package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	"testing"
)

func testHandler(t *testing.T) (*Handler, *store.Ledger) {
	t.Helper()
	ledger, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(qualification.NewService(ledger, protocol.NewEngine())), ledger
}

func TestWorkbenchAndCreateAPI(t *testing.T) {
	h, ledger := testHandler(t)
	defer ledger.Close()
	page := httptest.NewRecorder()
	h.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !bytes.Contains(page.Body.Bytes(), []byte("<body>")) {
		t.Fatalf("工作台响应无效: %d", page.Code)
	}
	body := map[string]any{"id": "web-a1", "lotCode": "LOT-1", "speciesName": "水稻", "harvestYear": 2025, "submittedQuantity": 100, "pretreatmentBoundary": "清水"}
	b, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/assessments", bytes.NewReader(b))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "create-1")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !bytes.Contains(response.Body.Bytes(), []byte(`"version":1`)) {
		t.Fatalf("创建响应无效: %d %s", response.Code, response.Body.String())
	}
}

func TestAPIRequiresIdempotencyKeyAndRejectsUnknownFields(t *testing.T) {
	h, ledger := testHandler(t)
	defer ledger.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/assessments", bytes.NewBufferString(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("未知字段应返回 400，得到 %d", response.Code)
	}
}

func TestObservationEndpointSupportsAtomicBatchAndPrecheck(t *testing.T) {
	h, ledger := testHandler(t)
	defer ledger.Close()
	ctx := context.Background()
	receipt, err := h.service.Create(ctx, "batch-create", qualification.CreateCommand{ID: "web-batch", LotCode: "LOT-B", SpeciesName: "水稻", HarvestYear: 2025, SubmittedQuantity: 200, PretreatmentBoundary: "清水"})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = h.service.FreezeProtocol(ctx, "web-batch", "batch-freeze", qualification.FreezeProtocolCommand{Versioned: qualification.Versioned{ExpectedVersion: receipt.Version}, ReplicateCount: 2, SeedsPerReplicate: 50, TemperatureMinC: 20, TemperatureMaxC: 30, ObservationDays: []int{3, 7}, TerminationDay: 7, MinimumGerminationRate: 80, MaximumDispersion: 10})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = h.service.PlaceReplicates(ctx, "web-batch", "batch-place", qualification.PlaceReplicatesCommand{Versioned: qualification.Versioned{ExpectedVersion: receipt.Version}, Replicates: []qualification.ReplicateInput{{ID: "r1", Label: "R1", SownQuantity: 50}, {ID: "r2", Label: "R2", SownQuantity: 50}}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = h.service.Start(ctx, "web-batch", "batch-start", qualification.StartCommand{Versioned: qualification.Versioned{ExpectedVersion: receipt.Version}})
	if err != nil {
		t.Fatal(err)
	}
	precheck := map[string]any{"expectedVersion": receipt.Version, "dayNo": 3, "recordedBy": "技术员", "validateOnly": true, "observations": []map[string]any{{"replicateId": "r1", "normalGerminated": 40, "abnormalSeedlings": 2, "deadSeeds": 1, "ungerminatedSeeds": 6}}}
	response := postJSON(t, h, "/api/assessments/web-batch/observations", "batch-precheck", precheck)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"valid":false`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"field":"hardSeeds"`)) {
		t.Fatalf("批量预检响应错误: %d %s", response.Code, response.Body.String())
	}
	batch := map[string]any{"expectedVersion": receipt.Version, "dayNo": 3, "recordedBy": "技术员", "observations": []map[string]any{
		{"replicateId": "r1", "normalGerminated": 40, "abnormalSeedlings": 2, "hardSeeds": 1, "deadSeeds": 1, "ungerminatedSeeds": 6},
		{"replicateId": "r2", "normalGerminated": 41, "abnormalSeedlings": 2, "hardSeeds": 1, "deadSeeds": 1, "ungerminatedSeeds": 5},
	}}
	response = postJSON(t, h, "/api/assessments/web-batch/observations", "batch-submit", batch)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"successfulGroups":2`)) || !bytes.Contains(response.Body.Bytes(), []byte(`"version":5`)) {
		t.Fatalf("批量提交响应错误: %d %s", response.Code, response.Body.String())
	}
}

func postJSON(t *testing.T, h *Handler, path, key string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	return response
}
