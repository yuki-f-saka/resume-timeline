package main

import (
	"embed"
	"fmt"
)

// The sample resume versions are embedded so that `-demo` works straight after
// `go install`, without cloning the repository or having a git repo at hand.
//
//go:embed examples/versions/*.pdf
var demoPDFs embed.FS

// mirrors the commits examples/demo.sh creates, so both demo paths tell the
// same story
var demoVersions = []Version{
	{Path: "examples/versions/v1.pdf", Date: "2026-01-05", Subject: "Initial resume"},
	{Path: "examples/versions/v2.pdf", Date: "2026-01-19", Subject: "Add audit pipeline bullet, mention high availability in summary"},
	{Path: "examples/versions/v3.pdf", Date: "2026-02-02", Subject: "Update peak traffic metric, drop legacy PHP bullet"},
	{Path: "examples/versions/v4.pdf", Date: "2026-02-16", Subject: "Add DynamoDB and Kubernetes to skills"},
}

// loadDemoVersions returns the built-in sample versions, oldest first.
func loadDemoVersions() ([]Version, error) {
	versions := make([]Version, len(demoVersions))
	for i, v := range demoVersions {
		content, err := demoPDFs.ReadFile(v.Path)
		if err != nil {
			return nil, fmt.Errorf("reading embedded sample %s: %w", v.Path, err)
		}
		v.PDF = content
		versions[i] = v
	}
	return versions, nil
}
