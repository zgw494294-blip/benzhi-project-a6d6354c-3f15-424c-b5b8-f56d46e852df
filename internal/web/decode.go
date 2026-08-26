package web

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

const maxRequestBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("Content-Type 必须是 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			return errors.New("请求体不能超过 1 MiB")
		}
		return errors.New("JSON 请求无效: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求体只能包含一个 JSON 对象")
	}
	return nil
}

func idempotencyKey(r *http.Request) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return "", errors.New("缺少 Idempotency-Key 请求头")
	}
	if len(key) > 160 {
		return "", errors.New("Idempotency-Key 不能超过 160 字符")
	}
	return key, nil
}
