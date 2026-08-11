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
	file := flag.String("file", "", "PDF file tracked in a git repository (required)")
	out := flag.String("out", "resume-timeline.html", "output HTML file")
	limit := flag.Int("limit", 0, "show only the N most recent versions (0 = all)")
	dpi := flag.Int("dpi", 150, "PDF rendering resolution")
	page := flag.Int("page", 1, "PDF page to compare")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: resume-timeline -file <path/to/file.pdf> [-out out.html] [-limit N] [-dpi N] [-page N]")
		os.Exit(2)
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		die("pdftoppm not found; install poppler (e.g. `brew install poppler` or `apt install poppler-utils`)")
	}

	abs, err := filepath.Abs(*file)
	if err != nil {
		die(err.Error())
	}
	// resolve symlinks (e.g. /var -> /private/var on macOS) so the path can be
	// made relative to the repository root git reports
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rootOut, err := runGit(filepath.Dir(abs), "rev-parse", "--show-toplevel")
	if err != nil {
		die(err.Error())
	}
	root := strings.TrimSpace(rootOut)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		die(err.Error())
	}

	versions, err := loadVersions(root, rel)
	if err != nil {
		die(err.Error())
	}
	if len(versions) == 0 {
		die("no versions found in git history for " + rel)
	}
	if *limit > 0 && len(versions) > *limit {
		versions = versions[len(versions)-*limit:]
	}

	tmp, err := os.MkdirTemp("", "resume-timeline")
	if err != nil {
		die(err.Error())
	}
	defer os.RemoveAll(tmp)

	pages := make([]*Page, len(versions))
	for i, v := range versions {
		p, err := renderPDF(tmp, i, *page, v.PDF, *dpi)
		if err != nil {
			die(fmt.Sprintf("rendering %s failed: %v", v.Short, err))
		}
		pages[i] = p
	}

	data := PageData{File: rel}
	for i, v := range versions {
		col := ColumnView{
			Index:   i + 1,
			Short:   v.Short,
			Date:    v.Date,
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
		fmt.Printf("v%-2d %s %s  bands=%-3d add=%-2d chg=%-2d del=%-2d  %s%s\n",
			i+1, v.Date, v.Short, len(pages[i].Bands),
			counts["add"], counts["chg"], counts["del"], truncate(v.Subject, 60), note)
	}

	if err := renderHTML(*out, data); err != nil {
		die(err.Error())
	}
	fmt.Printf("\n%d versions → %s\n", len(versions), *out)
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
