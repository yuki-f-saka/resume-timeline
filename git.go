package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type Version struct {
	Hash    string
	Short   string
	Date    string
	Subject string
	Path    string // path of the file as of this commit
	PDF     []byte
}

func runGit(dir string, args ...string) (string, error) {
	b, err := runGitBytes(dir, args...)
	return string(b), err
}

func runGitBytes(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

var headerRe = regexp.MustCompile(`^[0-9a-f]{40}\t`)
var statusRe = regexp.MustCompile(`^[AMRCDT]\d*\t`)

// loadVersions returns every committed version of the file, oldest first,
// following renames across the history.
func loadVersions(repo, rel string) ([]Version, error) {
	out, err := runGit(repo, "log", "--follow", "--name-status",
		"--format=%H%x09%ad%x09%s", "--date=short", "--", rel)
	if err != nil {
		return nil, err
	}

	var versions []Version
	var cur *Version
	for _, line := range strings.Split(out, "\n") {
		switch {
		case headerRe.MatchString(line):
			parts := strings.SplitN(line, "\t", 3)
			versions = append(versions, Version{
				Hash:    parts[0],
				Short:   parts[0][:7],
				Date:    parts[1],
				Subject: parts[2],
			})
			cur = &versions[len(versions)-1]
		case statusRe.MatchString(line) && cur != nil && cur.Path == "":
			// for renames (R100\told\tnew) the last field is the path at that commit
			fields := strings.Split(line, "\t")
			cur.Path = fields[len(fields)-1]
		}
	}

	// fetch the PDF blob of each version; skip commits where it is unavailable
	// (e.g. the commit that deleted the file)
	var loaded []Version
	for _, v := range versions {
		if v.Path == "" {
			continue
		}
		content, err := runGitBytes(repo, "show", v.Hash+":"+v.Path)
		if err != nil {
			continue
		}
		v.PDF = content
		loaded = append(loaded, v)
	}

	// git log emits newest first; reverse to oldest first
	for i, j := 0, len(loaded)-1; i < j; i, j = i+1, j-1 {
		loaded[i], loaded[j] = loaded[j], loaded[i]
	}
	return loaded, nil
}
