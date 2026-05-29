// config.go parses the hook2slack YAML configuration.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

// Config is the in-memory shape of the YAML configuration file.
type Config struct {
	Listen       string     // HTTP listen address
	SchemaFile   string     // path to the JSON Schema file
	TemplateFile string     // path to the Go text/template file
	Endpoints    []Endpoint // endpoint table
}

// Endpoint binds a URL path to a Slack incoming webhook URL. The
// destination channel is bound to the URL by Slack itself, so no
// per-endpoint channel field is carried here.
type Endpoint struct {
	Path string
	URL  string
}

// reservedPaths are paths the server uses for health endpoints.
// A configured endpoint cannot collide with these.
var reservedPaths = map[string]bool{
	"/-/healthy": true,
	"/-/ready":   true,
}

type configFile struct {
	Listen       string           `yaml:"listen"`
	SchemaFile   string           `yaml:"schema_file"`
	TemplateFile string           `yaml:"template_file"`
	Endpoints    []configEndpoint `yaml:"endpoints"`
}

type configEndpoint struct {
	Path  string              `yaml:"path"`
	Slack configEndpointSlack `yaml:"slack"`
}

type configEndpointSlack struct {
	URLFile string `yaml:"url_file"`
}

// LoadConfig reads a YAML file and returns a validated Config.
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cf configFile
	if err := yaml.Unmarshal(raw, &cf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if cf.Listen == "" {
		return nil, fmt.Errorf("listen is required")
	}
	if cf.SchemaFile == "" {
		return nil, fmt.Errorf("schema_file is required")
	}
	if cf.TemplateFile == "" {
		return nil, fmt.Errorf("template_file is required")
	}
	if len(cf.Endpoints) == 0 {
		return nil, fmt.Errorf("at least one endpoint is required")
	}

	cfg := &Config{
		Listen:       cf.Listen,
		SchemaFile:   cf.SchemaFile,
		TemplateFile: cf.TemplateFile,
	}

	seenPaths := map[string]bool{}
	for i, ce := range cf.Endpoints {
		if ce.Path == "" {
			return nil, fmt.Errorf("endpoint #%d: path is required", i+1)
		}
		if !strings.HasPrefix(ce.Path, "/") {
			return nil, fmt.Errorf("endpoint %q: path must start with /", ce.Path)
		}
		if reservedPaths[ce.Path] {
			return nil, fmt.Errorf("endpoint %q: path is reserved for server use", ce.Path)
		}
		if seenPaths[ce.Path] {
			return nil, fmt.Errorf("endpoint %q: duplicate path", ce.Path)
		}
		seenPaths[ce.Path] = true

		if ce.Slack.URLFile == "" {
			return nil, fmt.Errorf("endpoint %q: slack.url_file is required", ce.Path)
		}
		b, err := os.ReadFile(ce.Slack.URLFile)
		if err != nil {
			return nil, fmt.Errorf("endpoint %q: read url_file %s: %w", ce.Path, ce.Slack.URLFile, err)
		}

		cfg.Endpoints = append(cfg.Endpoints, Endpoint{
			Path: ce.Path,
			URL:  strings.TrimSpace(string(b)),
		})
	}

	return cfg, nil
}
