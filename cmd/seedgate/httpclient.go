package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

type checkClient struct {
	baseURL  string
	client   *http.Client
	sequence atomic.Int64
}

func (c *checkClient) post(ctx context.Context, path string, body any, target any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Idempotency-Key", fmt.Sprintf("selfcheck-%03d", c.sequence.Add(1)))
	return c.do(request, target)
}

func (c *checkClient) get(ctx context.Context, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	return c.do(request, target)
}

func (c *checkClient) do(request *http.Request, target any) error {
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s %s 返回 %d: %s", request.Method, request.URL.Path, response.StatusCode, b)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(target); err != nil {
		return fmt.Errorf("解析 %s: %w", request.URL.Path, err)
	}
	return nil
}
