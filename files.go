package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// loadFileVersions reads PDFs straight off disk, in the order given, for people
// whose old resumes are loose files rather than git history. The label it
// returns describes the set for the viewer heading.
func loadFileVersions(paths []string) ([]Version, string, error) {
	versions := make([]Version, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, "", err
		}
		if info.IsDir() {
			return nil, "", fmt.Errorf("%s is a directory; pass PDF files, or use -dir to take every PDF in it", p)
		}
		if !info.Mode().IsRegular() {
			return nil, "", fmt.Errorf("%s is not a regular file", p)
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, "", err
		}
		versions = append(versions, Version{
			Short: filepath.Base(p),
			Date:  info.ModTime().Format("2006-01-02"),
			Path:  p,
			PDF:   content,
		})
	}
	return versions, commonDir(paths), nil
}

// loadDirVersions takes every PDF directly inside dir, oldest first. File names
// in the wild ("resume_final2.pdf") say nothing reliable about order, so the
// modification time decides it, with the name breaking ties.
func loadDirVersions(dir string) ([]Version, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", err
	}

	type pdf struct {
		path    string
		modTime int64
		name    string
	}
	var pdfs []pdf
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, "", err
		}
		pdfs = append(pdfs, pdf{filepath.Join(dir, e.Name()), info.ModTime().UnixNano(), e.Name()})
	}
	if len(pdfs) == 0 {
		return nil, "", fmt.Errorf("no PDF files in %s", dir)
	}
	sort.Slice(pdfs, func(i, j int) bool {
		if pdfs[i].modTime != pdfs[j].modTime {
			return pdfs[i].modTime < pdfs[j].modTime
		}
		return pdfs[i].name < pdfs[j].name
	})

	paths := make([]string, len(pdfs))
	for i, p := range pdfs {
		paths[i] = p.path
	}
	return loadFileVersions(paths)
}

// commonDir names the set of inputs for the viewer heading: the directory they
// share, or a generic label when they are scattered.
func commonDir(paths []string) string {
	dir := ""
	for i, p := range paths {
		d := filepath.Dir(filepath.Clean(p))
		if i == 0 {
			dir = d
			continue
		}
		if d != dir {
			return "files"
		}
	}
	if dir == "" || dir == "." {
		return "files"
	}
	return dir
}

// printOrder shows the order the versions will be placed in before any
// rendering happens, so a wrong guess can be interrupted rather than waited out.
func printOrder(versions []Version) {
	fmt.Printf("using %d files (oldest first):\n", len(versions))
	for i, v := range versions {
		fmt.Printf("  v%-3d %s  %s\n", i+1, v.Date, v.Path)
	}
	fmt.Println()
}
