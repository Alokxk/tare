package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Alokxk/tare/cluster"
)

// Must stay a var: -ldflags "-X main.version=..." can only patch a string
// symbol, and a const would be inlined away and silently keep this default.
var version = "dev"

// Embedded rather than read from disk: a released binary travels alone, with
// no deploy/ directory beside it, and this keeps the repo file and the
// --print-rbac output from drifting apart.
//
//go:embed deploy/rbac.yaml
var rbacManifest string

type config struct {
	kubeconfig string
	context    string
	namespace  string
	out        string
	printRBAC  bool
}

func main() {
	flag.Usage = usage

	var cfg config
	flag.StringVar(&cfg.kubeconfig, "kubeconfig", "", "path to a kubeconfig file (default: $KUBECONFIG, then ~/.kube/config)")
	flag.StringVar(&cfg.context, "context", "", "kubeconfig context to use (default: the kubeconfig's current-context)")
	flag.StringVar(&cfg.namespace, "namespace", "", "namespace to analyse (default: all namespaces)")
	flag.StringVar(&cfg.out, "out", "", "also write an HTML report to this path")
	flag.BoolVar(&cfg.printRBAC, "print-rbac", false, "print the ClusterRole Tare needs and exit")
	showVersion := flag.Bool("version", false, "print the Tare version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("tare", version)
		return
	}

	// Ahead of every cluster call: the user who needs this is the one whose
	// connection is failing.
	if cfg.printRBAC {
		fmt.Print(rbacManifest)
		return
	}

	if err := validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "tare:", err)
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "tare:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	client, err := cluster.Connect(cluster.Options{
		KubeconfigPath: cfg.kubeconfig,
		Context:        cfg.context,
		UserAgent:      "tare/" + version,
	})
	if err != nil {
		return err
	}

	v, err := client.ServerVersion()
	if err != nil {
		return fmt.Errorf("reaching the API server: %w", err)
	}

	fmt.Printf("Connected to Kubernetes %s (%s/%s)\n", v.GitVersion, v.Platform, v.GoVersion)
	return nil
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
