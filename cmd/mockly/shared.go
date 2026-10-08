package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/configgen"
)

// loadResolvedConfig loads the config at path and resolves any inline
// schema reference it declares (HTTPConfig.OpenAPI, an AsyncAPI field, or a
// GRPCService.Proto) into mocks, printing any generator warning to stderr.
// This is the shared entry point for every command that actually needs the
// final, effective mock set (start, apply, config validate) — commands that
// only read e.g. the management API port don't need it.
func loadResolvedConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	warnings, err := configgen.Resolve(cfg, filepath.Dir(path))
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warn:", w)
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func postJSON(url string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(data))) // #nosec G107 -- URL from trusted config
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %d", resp.StatusCode)
	}
	return nil
}

func printResponse(resp *http.Response) {
	buf := make([]byte, 1<<20)
	n, _ := resp.Body.Read(buf)
	fmt.Println(string(buf[:n]))
}
