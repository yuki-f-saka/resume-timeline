package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const usage = `usage: resume-timeline [flags] <input>

  resume-timeline -file resume.pdf      every committed version, from git history
  resume-timeline v1.pdf v2.pdf v3.pdf  loose PDF files, in the order given
  resume-timeline -dir old-resumes/     every PDF in a directory, oldest first
  resume-timeline -demo                 a built-in sample, to see how it looks

flags must come before file arguments:
  resume-timeline -out timeline.html v1.pdf v2.pdf
`

func main() {
	file := flag.String("file", "", "PDF file tracked in a git repository")
	dir := flag.String("dir", "", "directory of PDF files to place on the timeline, oldest first")
	demo := flag.Bool("demo", false, "render the built-in sample timeline (no git repository needed)")
	out := flag.String("out", "resume-timeline.html", "output HTML file")
	limit := flag.Int("limit", 0, "show only the N most recent versions (0 = all)")
	dpi := flag.Int("dpi", 150, "PDF rendering resolution")
	page := flag.Int("page", 1, "PDF page to compare")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fmt.Fprintln(os.Stderr, "\nflags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	// the four input modes are alternatives; picking a winner silently would
	// hide the fact that one of them was ignored
	var modes []string
	if *file != "" {
		modes = append(modes, "-file")
	}
	if *dir != "" {
		modes = append(modes, "-dir")
	}
	if *demo {
		modes = append(modes, "-demo")
	}
	if flag.NArg() > 0 {
		modes = append(modes, "file arguments")
	}
	switch {
	case len(modes) == 0:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	case len(modes) > 1:
		die("pick one input, got " + strings.Join(modes, " and "))
	}

	if _, err := exec.LookPath("pdftoppm"); err != nil {
		die("pdftoppm not found; install poppler (e.g. `brew install poppler` or `apt install poppler-utils`)")
	}

	var (
		versions []Version
		label    string
		err      error
	)
	switch {
	case *demo:
		versions, err = loadDemoVersions()
		label = "demo · sample resume"
	case *file != "":
		versions, label, err = loadGitVersions(*file)
	case *dir != "":
		versions, label, err = loadDirVersions(*dir)
	default:
		versions, label, err = loadFileVersions(flag.Args())
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

	// loose files have no commit order to trust, so show the order that was
	// settled on while there is still time to interrupt
	if *file == "" && !*demo {
		printOrder(versions)
	}

	if err := buildTimeline(versions, label, *out, *page, *dpi); err != nil {
		die(err.Error())
	}
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
