package main

import (
	"encoding/base64"
	"image"
	"image/draw"
	"image/png"
	"strings"
)

const (
	inkThresh   = 180  // pixels darker than this (0-255 luminance) count as ink
	bandGap     = 3    // blank rows up to this height are merged into one band
	minBandH    = 3    // minimum band height in px
	gridW       = 96   // horizontal resolution of the band fingerprint
	simSame     = 0.90 // similarity at or above this means "same line"
	simChanged  = 0.30 // at or above this means "changed line", below means delete+add
	colDiffMin  = 0.15 // word-level: per-column ink density difference threshold
	boxMergeGap = 12   // word-level: boxes closer than this (px) are merged
	boxPad      = 4    // word-level: horizontal padding around each box (px)
	boxMinW     = 5    // word-level: minimum box width (px)
)

// Page is one rendered version with its detected text-line bands.
type Page struct {
	PNGB64 string
	W, H   int
	Img    image.Image
	Bands  []Band
}

// Band is a horizontal strip of the page corresponding to one text line.
type Band struct {
	Y0, Y1 int
	grid   []float64 // height-normalized ink density fingerprint
	cols   []float64 // per-column ink density (0..1)
	sum    float64
}

// Overlay is a diff highlight drawn over a column; coordinates are fractions
// of the page dimensions so they survive CSS scaling.
type Overlay struct {
	Kind     string // "add" | "chg" | "del"
	Top      float64
	Height   float64
	Boxes    []Box
	GhostB64 string // del: the removed line cropped from the previous version
}

type Box struct{ X0, W float64 }

func analyzePage(img image.Image) *Page {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ink := make([]bool, w*h)
	rowInk := make([]int, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum := (299*int(r>>8) + 587*int(g>>8) + 114*int(bl>>8)) / 1000
			if lum < inkThresh {
				ink[y*w+x] = true
				rowInk[y]++
			}
		}
	}

	p := &Page{W: w, H: h, Img: img}

	// detect runs of inked rows as bands, merging small gaps
	y := 0
	for y < h {
		if rowInk[y] < 2 {
			y++
			continue
		}
		y0 := y
		y1 := y
		gap := 0
		for yy := y + 1; yy < h; yy++ {
			if rowInk[yy] >= 2 {
				y1 = yy
				gap = 0
			} else {
				gap++
				if gap > bandGap {
					break
				}
			}
		}
		if y1-y0+1 >= minBandH {
			p.Bands = append(p.Bands, makeBand(ink, w, y0, y1))
		}
		y = y1 + gap + 1
	}
	return p
}

func makeBand(ink []bool, w, y0, y1 int) Band {
	bh := y1 - y0 + 1
	cols := make([]float64, w)
	for x := 0; x < w; x++ {
		c := 0
		for y := y0; y <= y1; y++ {
			if ink[y*w+x] {
				c++
			}
		}
		cols[x] = float64(c) / float64(bh)
	}
	grid := make([]float64, gridW)
	sum := 0.0
	for x := 0; x < w; x++ {
		grid[x*gridW/w] += cols[x]
		sum += cols[x]
	}
	return Band{Y0: y0, Y1: y1, grid: grid, cols: cols, sum: sum}
}

// bandSim returns the similarity (0..1) of two bands; identical lines are
// close to 1 even when they sit at different vertical positions.
func bandSim(a, b *Band) float64 {
	if a.sum == 0 && b.sum == 0 {
		return 1
	}
	diff := 0.0
	for k := 0; k < gridW; k++ {
		d := a.grid[k] - b.grid[k]
		if d < 0 {
			d = -d
		}
		diff += d
	}
	return 1 - diff/(a.sum+b.sum+1e-9)
}

// maxUpgradeRatio caps how many LCS-aligned "same" bands may be upgraded to
// "changed" by the word-level pass. When more than this fraction of aligned
// bands shows word-level differences, the cause is almost certainly a global
// rendering drift (fonts, metrics) between the two PDFs rather than edits, and
// the upgrades are dropped to keep the diff readable. Reported via drift.
const maxUpgradeRatio = 0.5

// diffPages aligns the bands of two versions with an LCS and produces the
// overlays for the newer one.
func diffPages(prev, cur *Page) (overlays []Overlay, drift bool) {
	n, m := len(prev.Bands), len(cur.Bands)
	// dp[i][j] = LCS length of prev[i:] and cur[j:]
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if bandSim(&prev.Bands[i], &cur.Bands[j]) >= simSame {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	type op struct {
		kind int // 0=same 1=del 2=add
		i, j int
	}
	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		if bandSim(&prev.Bands[i], &cur.Bands[j]) >= simSame {
			ops = append(ops, op{0, i, j})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, op{1, i, j})
			i++
		} else {
			ops = append(ops, op{2, i, j})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{1, i, m})
	}
	for ; j < m; j++ {
		ops = append(ops, op{2, n, j})
	}

	H := float64(cur.H)
	var upgrades []Overlay
	sameCount := 0
	lastBottom := 0

	addOverlay := func(b *Band) {
		overlays = append(overlays, Overlay{
			Kind: "add", Top: float64(b.Y0-1) / H, Height: float64(b.Y1-b.Y0+3) / H,
		})
		lastBottom = b.Y1
	}
	delMarker := func(pb *Band) {
		overlays = append(overlays, Overlay{
			Kind: "del", Top: float64(lastBottom+2) / H, Height: 4 / H,
			GhostB64: cropB64(prev.Img, pb.Y0, pb.Y1),
		})
	}
	chgOverlay := func(pb, cb *Band) {
		overlays = append(overlays, Overlay{
			Kind: "chg", Top: float64(cb.Y0-1) / H, Height: float64(cb.Y1-cb.Y0+3) / H,
			Boxes: changedBoxes(pb, cb, cur.W),
		})
		lastBottom = cb.Y1
	}

	k := 0
	for k < len(ops) {
		if ops[k].kind == 0 {
			// lines aligned by the LCS can still contain small edits (a number,
			// a single word) that keep similarity above simSame; a word-level
			// pass over the aligned pair catches those
			pb, cb := &prev.Bands[ops[k].i], &cur.Bands[ops[k].j]
			sameCount++
			if boxes := changedBoxes(pb, cb, cur.W); len(boxes) > 0 {
				upgrades = append(upgrades, Overlay{
					Kind: "chg", Top: float64(cb.Y0-1) / H, Height: float64(cb.Y1-cb.Y0+3) / H,
					Boxes: boxes,
				})
			}
			lastBottom = cb.Y1
			k++
			continue
		}
		var dels, adds []int
		for k < len(ops) && ops[k].kind != 0 {
			if ops[k].kind == 1 {
				dels = append(dels, ops[k].i)
			} else {
				adds = append(adds, ops[k].j)
			}
			k++
		}
		p := 0
		for ; p < len(dels) && p < len(adds); p++ {
			pb, cb := &prev.Bands[dels[p]], &cur.Bands[adds[p]]
			if bandSim(pb, cb) >= simChanged {
				chgOverlay(pb, cb)
			} else {
				delMarker(pb)
				addOverlay(cb)
			}
		}
		for ; p < len(dels); p++ {
			delMarker(&prev.Bands[dels[p]])
		}
		for ; p < len(adds); p++ {
			addOverlay(&cur.Bands[adds[p]])
		}
	}

	if float64(len(upgrades)) <= maxUpgradeRatio*float64(sameCount) {
		overlays = append(overlays, upgrades...)
	} else {
		drift = true
	}
	return overlays, drift
}

// changedBoxes finds the x-ranges that differ inside a changed line
// (the word-level part of the diff).
func changedBoxes(pb, cb *Band, w int) []Box {
	// page widths can differ slightly between versions; clamp to the narrower one
	w = min(w, min(len(pb.cols), len(cb.cols)))
	W := float64(w)
	diff := make([]bool, w)
	for x := 0; x < w; x++ {
		d := pb.cols[x] - cb.cols[x]
		if d < 0 {
			d = -d
		}
		diff[x] = d > colDiffMin
	}
	var boxes []Box
	x := 0
	for x < w {
		if !diff[x] {
			x++
			continue
		}
		x0 := x
		x1 := x
		gap := 0
		for xx := x + 1; xx < w; xx++ {
			if diff[xx] {
				x1 = xx
				gap = 0
			} else {
				gap++
				if gap > boxMergeGap {
					break
				}
			}
		}
		if x1-x0+1 >= boxMinW {
			left := max(0, x0-boxPad)
			right := min(w-1, x1+boxPad)
			boxes = append(boxes, Box{X0: float64(left) / W, W: float64(right-left+1) / W})
		}
		x = x1 + gap + 1
	}
	return boxes
}

// cropB64 crops a y-range out of an image and returns it as base64 PNG
// (used for the hover preview of deleted lines).
func cropB64(img image.Image, y0, y1 int) string {
	b := img.Bounds()
	pad := 2
	r := image.Rect(b.Min.X, b.Min.Y+max(0, y0-pad), b.Max.X, b.Min.Y+min(b.Dy(), y1+pad+1))
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	var buf strings.Builder
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := png.Encode(enc, dst); err != nil {
		return ""
	}
	enc.Close()
	return buf.String()
}
