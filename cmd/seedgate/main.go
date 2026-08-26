package main

import (
	"fmt"
	"os"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err == nil {
		if cfg.mode == "selfcheck" {
			err = selfcheck(cfg)
		} else {
			err = serve(cfg)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "seedgate:", err)
		os.Exit(1)
	}
}
