package configexample

import _ "embed"

// Content holds the content of config.example.yaml embedded at build time.
//
//go:embed config.example.yaml
var Content string
