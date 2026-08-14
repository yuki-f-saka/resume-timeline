package main

import (
	"fmt"
	"strings"
)

// Version is one version of the document to place on the timeline. It is not
// tied to git: the hash fields are simply empty for versions that came from
// loose files or the built-in demo.
type Version struct {
	Hash    string
	Short   string
	Date    string
	Subject string
	Path    string // path of the file as of this commit, or as given on the command line
	PDF     []byte
}

// meta is the column heading for the i-th version (0-based). Versions that did
// not come from git carry no hash, so empty fields are dropped rather than
// leaving stray separators.
func (v Version) meta(i int) string {
	parts := []string{fmt.Sprintf("v%d", i+1)}
	for _, s := range []string{v.Date, v.Short} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}
