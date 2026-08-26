package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"seed-vigor-gate/internal/protocol"
	"seed-vigor-gate/internal/qualification"
	"seed-vigor-gate/internal/store"
	webui "seed-vigor-gate/internal/web"
	"syscall"
	"time"
)

type application struct {
	ledger  *store.Ledger
	service *qualification.Service
	handler http.Handler
}

func assemble(dataDir string) (*application, error) {
	ledger, err := store.Open(dataDir)
	if err != nil {
		return nil, err
	}
	service := qualification.NewService(ledger, protocol.NewEngine())
	return &application{ledger: ledger, service: service, handler: webui.NewHandler(service)}, nil
}

func (a *application) close() error { return a.ledger.Close() }

func productionServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
}

func serve(cfg config) error {
	app, err := assemble(cfg.dataDir)
	if err != nil {
		return fmt.Errorf("初始化应用: %w", err)
	}
	defer app.close()
	listener, err := net.Listen("tcp", cfg.address)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.address, err)
	}
	server := productionServer(cfg.address, app.handler)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	slog.Info("种子发芽资格评定服务已启动", "address", "http://"+cfg.address, "data", cfg.dataDir)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		slog.Info("收到关闭信号", "signal", sig.String())
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
