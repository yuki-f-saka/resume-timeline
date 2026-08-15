package main

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// PDF rasterizing runs PDFium compiled to WebAssembly on wazero, a pure-Go
// runtime. That keeps the whole tool a single binary with nothing to install
// alongside it: no poppler, no cgo, nothing left on the machine afterwards.

var (
	pdfiumOnce sync.Once
	pdfiumInst pdfium.Pdfium
	pdfiumErr  error
)

// instance starts PDFium on first use. Starting it costs a moment, so it is
// shared for the whole run rather than created per version.
func instance() (pdfium.Pdfium, error) {
	pdfiumOnce.Do(func() {
		pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
		if err != nil {
			pdfiumErr = fmt.Errorf("starting the PDF renderer: %w", err)
			return
		}
		pdfiumInst, err = pool.GetInstance(time.Minute)
		if err != nil {
			pdfiumErr = fmt.Errorf("starting the PDF renderer: %w", err)
		}
	})
	return pdfiumInst, pdfiumErr
}

// renderPDF rasterizes one page of the PDF and analyzes it.
func renderPDF(page int, pdf []byte, dpi int) (*Page, error) {
	inst, err := instance()
	if err != nil {
		return nil, err
	}

	doc, err := inst.OpenDocument(&requests.OpenDocument{File: &pdf})
	if err != nil {
		return nil, err
	}
	defer inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	res, err := inst.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI: dpi,
		Page: requests.Page{ByIndex: &requests.PageByIndex{
			Document: doc.Document,
			Index:    page - 1, // -page is 1-based, PDFium is 0-based
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("rendering page %d: %w", page, err)
	}

	// under WebAssembly the returned pixel buffer is only valid until Cleanup,
	// but the image is kept for the whole run (deleted lines are cropped out of
	// it later), so take a copy of our own before releasing it
	img := clone(res.Result.RenderedImage)
	res.Cleanup()

	b64, err := encodePNG(img)
	if err != nil {
		return nil, err
	}
	p := analyzePage(img)
	p.PNGB64 = b64
	return p, nil
}

func clone(src image.Image) image.Image {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

func encodePNG(img image.Image) (string, error) {
	var buf strings.Builder
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	if err := png.Encode(enc, img); err != nil {
		return "", err
	}
	enc.Close()
	return buf.String(), nil
}
