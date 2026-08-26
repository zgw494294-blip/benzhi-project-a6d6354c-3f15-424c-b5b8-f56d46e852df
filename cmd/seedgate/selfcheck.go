package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"seed-vigor-gate/internal/qualification"
	"time"
)

type commandEnvelope struct {
	Assessment qualification.AssessmentView `json:"assessment"`
}

func selfcheck(cfg config) error {
	tempDir, err := os.MkdirTemp("", "seedgate-selfcheck-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	app, err := assemble(tempDir)
	if err != nil {
		return fmt.Errorf("装配 selfcheck: %w", err)
	}
	defer app.close()
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("selfcheck 监听 %s: %w", cfg.address, err)
	}
	server := productionServer(cfg.address, app.handler)
	serverError := make(chan error, 1)
	go func() { serverError <- server.Serve(listener) }()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	client := &checkClient{baseURL: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 4 * time.Second}}
	if err := runNormalFlow(ctx, client); err != nil {
		_ = server.Shutdown(context.Background())
		return err
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serverError; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	if err := app.close(); err != nil {
		return err
	}
	return verifyRecovery(tempDir)
}

func runNormalFlow(ctx context.Context, client *checkClient) error {
	id := "selfcheck-assessment"
	var result commandEnvelope
	if err := client.post(ctx, "/api/assessments", map[string]any{"id": id, "lotCode": "SELF-LOT-001", "speciesName": "水稻", "harvestYear": 2025, "submittedQuantity": 100, "pretreatmentBoundary": "清水浸种 24 小时"}, &result); err != nil {
		return err
	}
	version := result.Assessment.Assessment.Version
	steps := []struct {
		path string
		body func(int64) any
	}{
		{"/protocol/freeze", func(v int64) any {
			return map[string]any{"expectedVersion": v, "replicateCount": 2, "seedsPerReplicate": 50, "temperatureMinC": 20, "temperatureMaxC": 30, "observationDays": []int{3, 7}, "terminationDay": 7, "minimumGerminationRate": 80, "maximumDispersion": 10}
		}},
		{"/replicates", func(v int64) any {
			return map[string]any{"expectedVersion": v, "replicates": []map[string]any{{"id": "r1", "label": "R1", "sownQuantity": 50}, {"id": "r2", "label": "R2", "sownQuantity": 50}}}
		}},
		{"/start", func(v int64) any { return map[string]any{"expectedVersion": v} }},
		{"/observations", observationBody("r1", 3, 42, 4)},
		{"/observations", observationBody("r2", 3, 41, 5)},
		{"/observations", observationBody("r1", 7, 46, 0)},
		{"/observations", observationBody("r2", 7, 45, 1)},
		{"/calculate", func(v int64) any { return map[string]any{"expectedVersion": v} }},
		{"/review/approve", func(v int64) any {
			return map[string]any{"expectedVersion": v, "reviewer": "selfcheck-reviewer", "reason": "正常流程证据完整"}
		}},
		{"/seal", func(v int64) any { return map[string]any{"expectedVersion": v} }},
	}
	for _, step := range steps {
		if err := client.post(ctx, "/api/assessments/"+id+step.path, step.body(version), &result); err != nil {
			return err
		}
		version = result.Assessment.Assessment.Version
	}
	certificate := result.Assessment.Certificate
	if certificate == nil {
		return errors.New("selfcheck 未生成资格凭据")
	}
	var verification qualification.CertificateVerification
	if err := client.get(ctx, "/api/certificates/"+url.PathEscape(certificate.CertificateNo)+"/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.Certificate.Decision != "qualified" {
		return errors.New("selfcheck 凭据校验或资格结论不正确")
	}
	fmt.Printf("selfcheck 通过：评定 %s，凭据 %s，版本 %d\n", id, certificate.CertificateNo, version)
	return nil
}

func observationBody(replicate string, day, normal, ungerminated int) func(int64) any {
	return func(version int64) any {
		return map[string]any{"expectedVersion": version, "replicateId": replicate, "dayNo": day, "normalGerminated": normal, "abnormalSeedlings": 2, "hardSeeds": 1, "deadSeeds": 1, "ungerminatedSeeds": ungerminated, "recordedBy": "selfcheck-technician"}
	}
}

func verifyRecovery(dir string) error {
	app, err := assemble(dir)
	if err != nil {
		return fmt.Errorf("重启恢复失败: %w", err)
	}
	defer app.close()
	view, err := app.service.Get(context.Background(), "selfcheck-assessment")
	if err != nil {
		return err
	}
	if view.Assessment.Status != "sealed" || view.Certificate == nil {
		return errors.New("重启后封存状态或凭据丢失")
	}
	verification, err := app.service.VerifyCertificate(context.Background(), view.Certificate.CertificateNo)
	if err != nil || !verification.Valid {
		return errors.New("重启后凭据摘要校验失败")
	}
	return nil
}
