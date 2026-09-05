package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Must stay a var: -ldflags "-X main.version=..." can only patch a string
// symbol, and a const would be inlined away and silently keep this default.
var version = "dev"

type config struct {
	kubeconfig string
	context    string
	namespace  string
	out        string
}

func main() {
	flag.Usage = usage

	var cfg config
	flag.StringVar(&cfg.kubeconfig, "kubeconfig", "", "path to a kubeconfig file (default: $KUBECONFIG, then ~/.kube/config)")
	flag.StringVar(&cfg.context, "context", "", "kubeconfig context to use (default: the kubeconfig's current-context)")
	flag.StringVar(&cfg.namespace, "namespace", "", "namespace to analyse (default: all namespaces)")
	flag.StringVar(&cfg.out, "out", "", "also write an HTML report to this path")
	showVersion := flag.Bool("version", false, "print the Tare version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("tare", version)
		return
	}

	if err := validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "tare:", err)
		os.Exit(2)
	}

	flag.Usage()
	os.Exit(2)
}

// Runs before any cluster work so an unwritable --out fails immediately
// instead of after the metrics sampling window.
func validate(cfg config) error {
	if cfg.out == "" {
		return nil
	}

	dir := filepath.Dir(cfg.out)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("--out %s: %w", cfg.out, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--out %s: %s is not a directory", cfg.out, dir)
	}
	return nil
}

func usage() {
	fmt.Fprint(flag.CommandLine.Output(), `Tare - find out if Kubernetes is worth it.

A read-only CLI that reports real usage and real feature adoption on your
Kubernetes cluster.

Usage:
  tare [flags]

Flags:
`)
	flag.PrintDefaults()
}
