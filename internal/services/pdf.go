package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ledongthuc/pdf"
)

type PDFService struct{}

func NewPDFService() *PDFService {
	return &PDFService{}
}

func (s *PDFService) ReadPDFBytes(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	return bytes, nil
}

// PDFPageImage represents a single page converted to an image
type PDFPageImage struct {
	PageNumber int
	ImageData  string // base64 encoded image with data URI prefix
}

// ConvertPDFPagesToImages converts each page of a PDF to a base64-encoded PNG image
// Uses Ghostscript for proper PDF rendering
func (s *PDFService) ConvertPDFPagesToImages(path string) ([]PDFPageImage, error) {
	// First, get the number of pages using the pdf library
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf for page count: %w", err)
	}
	numPages := r.NumPage()
	f.Close()

	if numPages == 0 {
		return nil, fmt.Errorf("pdf has no pages")
	}

	// Create a temporary directory for rendered images
	tempDir, err := os.MkdirTemp("", "pdf-render-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Use Ghostscript to render all pages at once
	// -dNOPAUSE -dBATCH: non-interactive mode
	// -sDEVICE=png16m: 24-bit color PNG
	// -r150: 150 DPI resolution (good balance between quality and size)
	// -sOutputFile: output pattern with %d for page numbers
	outputPattern := filepath.Join(tempDir, "page-%03d.png")
	cmd := exec.Command("gs",
		"-dQUIET",
		"-dSAFER",
		"-dNOPAUSE",
		"-dBATCH",
		"-sDEVICE=png16m",
		"-r150",
		fmt.Sprintf("-sOutputFile=%s", outputPattern),
		path,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ghostscript render failed: %w, stderr: %s", err, stderr.String())
	}

	// Read each rendered page and convert to base64
	var pages []PDFPageImage
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		// Ghostscript uses 1-based numbering in output
		pagePath := filepath.Join(tempDir, fmt.Sprintf("page-%03d.png", pageNum))

		// Read the rendered PNG file
		imageData, err := os.ReadFile(pagePath)
		if err != nil {
			return nil, fmt.Errorf("read rendered page %d: %w", pageNum, err)
		}

		// Encode to base64 data URI
		base64Data := base64.StdEncoding.EncodeToString(imageData)
		dataURI := "data:image/png;base64," + base64Data

		pages = append(pages, PDFPageImage{
			PageNumber: pageNum,
			ImageData:  dataURI,
		})
	}

	return pages, nil
}
