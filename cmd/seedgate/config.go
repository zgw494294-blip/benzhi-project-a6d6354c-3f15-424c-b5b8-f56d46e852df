package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	mode    string
	address string
	dataDir string
}

func parseConfig(args []string) (config, error) {
	cfg := config{mode: "serve", address: addressFromEnvironment(), dataDir: "./data"}
	if len(args) > 0 && args[0] == "selfcheck" {
		cfg.mode = "selfcheck"
		args = args[1:]
	}
	set := flag.NewFlagSet("seedgate", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	set.StringVar(&cfg.address, "addr", cfg.address, "回环监听地址，例如 127.0.0.1:19081")
	set.StringVar(&cfg.dataDir, "data", cfg.dataDir, "本地事件账本目录")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("未知参数: %s", strings.Join(set.Args(), " "))
	}
	if err := validateAddress(cfg.address); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func addressFromEnvironment() string {
	value := strings.TrimSpace(os.Getenv("PORT"))
	if value == "" {
		return defaultAddress
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1024 || port > 65535 {
		return defaultAddress
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须是明确的 host:port: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return errors.New("-addr 仅允许回环地址 127.0.0.1、localhost 或 ::1")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return errors.New("监听端口必须是 1024–65535 的明确端口")
	}
	if port == 3000 || port == 8080 {
		return errors.New("请使用高位、非惯用开发端口")
	}
	return nil
}
