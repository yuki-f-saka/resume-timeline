package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	file := flag.String("file", "", "PDF file tracked in a git repository (required unless -demo)")
	demo := flag.Bool("demo", false, "render the built-in sample timeline (no git repository needed)")
	out := flag.String("out", "resume-timeline.html", "output HTML file")
	limit := flag.Int("limit", 0, "show only the N most recent versions (0 = all)")
	dpi := flag.Int("dpi", 150, "PDF rendering resolution")
	page := flag.Int("page", 1, "PDF page to compare")
	flag.Parse()

	if *file == "" && !*demo {
		fmt.Fprintln(os.Stderr, "usage: resume-timeline -file <path/to/file.pdf> [-out out.html] [-limit N] [-dpi N] [-page N]")
		fmt.Fprintln(os.Stderr, "       resume-timeline -demo    # try it without a git repository")
		os.Exit(2)
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		die("pdftoppm not found; install poppler (e.g. `brew install poppler` or `apt install poppler-utils`)")
	}

	var (
		versions []Version
		label    string
		err      error
	)
	if *demo {
		versions, err = loadDemoVersions()
		label = "demo · sample resume"
	} else {
		versions, label, err = loadGitVersions(*file)
	}
	if err != nil {
		die(err.Error())
	}
	if len(versions) == 0 {
		die("no versions found in git history for " + label)
	}

	if *limit > 0 && len(versions) > *limit {
		versions = versions[len(versions)-*limit:]
	}

	if err := buildTimeline(versions, label, *out, *page, *dpi); err != nil {
		die(err.Error())
	}
}

// loadGitVersions resolves the file to its repository and returns every
// committed version of it, oldest first, along with its repo-relative path.
func loadGitVersions(file string) ([]Version, string, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, "", err
	}
	// resolve symlinks (e.g. /var -> /private/var on macOS) so the path can be
	// made relative to the repository root git reports
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rootOut, err := runGit(filepath.Dir(abs), "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, "", err
	}
	root := strings.TrimSpace(rootOut)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, "", err
	}
	versions, err := loadVersions(root, rel)
	if err != nil {
		return nil, "", err
	}
	return versions, rel, nil
}

// buildTimeline rasterizes every version, diffs adjacent pairs and writes the
// viewer. It is agnostic about where the versions came from.
func buildTimeline(versions []Version, label, out string, page, dpi int) error {
	tmp, err := os.MkdirTemp("", "resume-timeline")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	pages := make([]*Page, len(versions))
	for i, v := range versions {
		p, err := renderPDF(tmp, i, page, v.PDF, dpi)
		if err != nil {
			return fmt.Errorf("rendering %s failed: %v", v.meta(i), err)
		}
		pages[i] = p
	}

	data := PageData{File: label}
	for i, v := range versions {
		col := ColumnView{
			Meta:    v.meta(i),
			Subject: v.Subject,
			PNGB64:  pages[i].PNGB64,
		}
		drift := false
		if i > 0 {
			col.Overlays, drift = diffPages(pages[i-1], pages[i])
		}
		data.Cols = append(data.Cols, col)

		counts := map[string]int{}
		for _, o := range col.Overlays {
			counts[o.Kind]++
		}
		note := ""
		if drift {
			note = " [rendering drift: word-level pass skipped]"
		}
		fmt.Printf("%-26s bands=%-3d add=%-2d chg=%-2d del=%-2d  %s%s\n",
			v.meta(i), len(pages[i].Bands),
			counts["add"], counts["chg"], counts["del"], truncate(v.Subject, 60), note)
	}

	if err := renderHTML(out, data); err != nil {
		return err
	}
	fmt.Printf("\n%d versions → %s\n", len(versions), out)
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}
