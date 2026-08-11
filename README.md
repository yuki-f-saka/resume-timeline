# resume-timeline

Visual diff timeline for PDF files tracked in git — see how your resume
evolved, commit by commit, side by side.

![demo](docs/demo.png)

Git can show you how the *source* of your resume changed, but not how the
**rendered page** changed. `resume-timeline` pulls every committed version of
a PDF out of your git history, renders them into one horizontally scrollable
page (oldest → newest), and overlays GitHub-style diff highlights directly on
the page image:

- **Added lines** — light green band
- **Changed lines** — the regions that actually changed are boxed in darker green
- **Deleted lines** — red marker; hover it to peek at the removed content

Everything is emitted as a single self-contained HTML file. No server, no
dependencies at view time — just open it in a browser.

## Install

Requires Go 1.22+ and [poppler](https://poppler.freedesktop.org/) (`pdftoppm`):

```bash
brew install poppler          # macOS
sudo apt install poppler-utils  # Debian/Ubuntu

go install github.com/yuki-f-saka/resume-timeline@latest
```

## Usage

```bash
resume-timeline -file path/to/resume.pdf -out timeline.html
open timeline.html
```

| Flag | Default | Description |
|---|---|---|
| `-file` | (required) | PDF file tracked in a git repository |
| `-out` | `resume-timeline.html` | output HTML file |
| `-limit` | `0` (all) | show only the N most recent versions |
| `-dpi` | `150` | rendering resolution |
| `-page` | `1` | PDF page to compare |

The viewer opens with the two most recent versions side by side. Drag the
column-width slider to zoom out to five or more versions at once; vertical
scrolling is synchronized across columns so corresponding lines stay aligned
(toggle it off in the toolbar).

Try it without your own history:

```bash
git clone https://github.com/yuki-f-saka/resume-timeline
cd resume-timeline
./examples/demo.sh   # builds a throwaway git repo with 4 sample versions
open demo-timeline.html
```

## How it works

Diffing rendered PDFs is not a text problem — and a naive pixel diff fails
the moment one added line shifts everything below it. Instead:

1. Every committed version of the PDF is collected via `git log --follow`
   (renames are tracked) and rasterized with `pdftoppm`.
2. Rows of ink are segmented into **bands** — horizontal strips corresponding
   to text lines — using the page's ink-density profile.
3. Adjacent versions are aligned band-by-band with an **LCS** over normalized
   band fingerprints. A line that merely moved down is matched to itself, so
   layout shifts don't pollute the diff.
4. Unmatched bands become additions/deletions; near-matches become "changed"
   lines, and a column-wise ink comparison inside each changed pair marks the
   word-level regions that actually differ.
5. Aligned pairs also get the word-level pass, so a single edited number in an
   otherwise identical line is still caught. If that pass flags most of the
   page, the two PDFs were rendered with drifted font metrics rather than
   edited, and it is skipped for that pair (reported on stdout).

The overlays are positioned in relative coordinates and drawn on top of the
page images in plain HTML/CSS.

## Limitations

- Compares one page at a time (`-page`); multi-column layouts within a page
  are handled only as full-width bands.
- Band alignment assumes horizontally stable text (no per-line reflow of the
  whole page). Works best for documents like resumes that keep a consistent
  layout between versions.

## License

[MIT](LICENSE)
