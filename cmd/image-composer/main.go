package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/config"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/pkgfetcher"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
	_ "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider/azurelinux3" // register provider
	_ "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider/elxr12"      // register provider
	_ "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider/emt3_0"      // register provider
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
)

// temporary placeholder for configuration
// This should be replaced with a proper configuration struct
const (
	workers = 8
	destDir = "./downloads"
)

// nopSyncer wraps an io.Writer but its Sync() does nothing.
type nopSyncer struct{ io.Writer }
func (n nopSyncer) Sync() error { return nil }

// setupLogger initializes a zap logger with development config,
// but replaces the usual fsyncing writer with one whose Sync() is a no-op.
func setupLogger() (*zap.Logger, error) {
    // start from DevConfig so we get console output, color, ISO8601 time, etc.
    cfg := zap.NewDevelopmentConfig()
    cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    cfg.EncoderConfig.EncodeTime  = zapcore.ISO8601TimeEncoder
    cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

    // create a console encoder using your EncoderConfig
    encoder := zapcore.NewConsoleEncoder(cfg.EncoderConfig)
    // wrap stderr in our nopSyncer
    writer  := nopSyncer{os.Stderr}
    // build a core that writes to that writer
    core    := zapcore.NewCore(encoder, writer, cfg.Level)

    // mirror the options NewDevelopmentConfig() would have added
    opts := []zap.Option{
        zap.AddCaller(),
        zap.Development(),
        zap.AddStacktrace(zapcore.ErrorLevel),
    }

    return zap.New(core, opts...), nil
}
func main() {

	logger, err := setupLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()            // never errors
	zap.ReplaceGlobals(logger)
	sugar := zap.S()

	// check for input JSON
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.json>\n", os.Args[0])
		os.Exit(1)
	}
	configPath := os.Args[1]

	bc, err := config.Load(configPath)
	if err != nil {
		sugar.Fatalf("loading config: %v", err)
	}

	providerName := bc.Distro + bc.Version

	// get provider by name
	p, ok := provider.Get(providerName)
	if !ok {
		sugar.Fatalf("provider not found, %s", providerName)
	}

	// initialize provider
	if err := p.Init(bc); err != nil {
		sugar.Fatalf("provider init: %v", err)
	}

	// fetch the entire package list
	all, err := p.Packages()
	if err != nil {
		sugar.Fatalf("getting packages: %v", err)
	}

	// match the packages in the build spec against all the packages
	req, err := p.MatchRequested(bc.Packages, all)
	if err != nil {
		sugar.Fatalf("matching packages: %v", err)
	}
	sugar.Infof("matched al total of %d packages", len(req))
	for _, pkg := range req {
		sugar.Infof("-> %s", pkg.Name)
	}
	
	// resolve the dependencies of the requested packages
	needed, err := p.Resolve(req, all)
	if err != nil {
		sugar.Fatalf("resolving packages: %v", err)
	}
	sugar.Infof("resolved %d packages", len(needed))


	// extract URLs
	urls := make([]string, len(needed))
	for i, pkg := range needed {
		urls[i] = pkg.URL
	}

	// populate the cache download
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		sugar.Fatalf("invalid dest: %v", err)
	}
	sugar.Infof("downloading %d packages to %s", len(urls), absDest)
	if err := pkgfetcher.FetchPackages(urls, absDest, workers); err != nil {
		sugar.Fatalf("fetch failed: %v", err)
	}
	sugar.Info("all downloads complete")

	// verify downloaded packages
	if err := p.Validate(destDir); err != nil {
		sugar.Fatalf("verification failed: %v", err)
	}

}
