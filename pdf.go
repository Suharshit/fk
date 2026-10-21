package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ---------- images -> PDF (pure Go, no tools needed) ----------

type pdfImage struct {
	w, h   int
	cs     string // DeviceRGB or DeviceGray
	filter string // DCTDecode or FlateDecode
	data   []byte
}

func preparePDFImage(path string) (*pdfImage, error) {
	// JPEGs that are already upright go in as they are: no quality loss.
	if kindOf(path) == KindJPEG && jpegOrientation(path) == 1 {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
		if err == nil && (cfg.ColorModel == color.YCbCrModel || cfg.ColorModel == color.GrayModel) {
			cs := "DeviceRGB"
			if cfg.ColorModel == color.GrayModel {
				cs = "DeviceGray"
			}
			return &pdfImage{cfg.Width, cfg.Height, cs, "DCTDecode", raw}, nil
		}
	}

	img, err := loadImage(path)
	if err != nil {
		return nil, err
	}
	img = flatten(img)
	b := img.Bounds()

	if kindOf(path) == KindJPEG { // rotated or CMYK JPEG: re-encode as JPEG
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
			return nil, err
		}
		return &pdfImage{b.Dx(), b.Dy(), "DeviceRGB", "DCTDecode", buf.Bytes()}, nil
	}

	// PNG and others: lossless RGB, zlib compressed
	n := toNRGBA(img)
	rgb := make([]byte, 0, b.Dx()*b.Dy()*3)
	for i := 0; i < len(n.Pix); i += 4 {
		rgb = append(rgb, n.Pix[i], n.Pix[i+1], n.Pix[i+2])
	}
	var buf bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	zw.Write(rgb)
	zw.Close()
	return &pdfImage{b.Dx(), b.Dy(), "DeviceRGB", "FlateDecode", buf.Bytes()}, nil
}

// imagesToPDF writes one page per image. Pages are sized so the image shows
// at 96 DPI, like a browser would show it.
func imagesToPDF(out string, paths []string) error {
	imgs := make([]*pdfImage, 0, len(paths))
	for _, p := range paths {
		im, err := preparePDFImage(p)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
		imgs = append(imgs, im)
	}

	var buf bytes.Buffer
	offsets := []int{0} // object 0 is the free entry
	obj := func(body func()) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n", len(offsets)-1)
		body()
		buf.WriteString("\nendobj\n")
	}

	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	// 1: catalog, 2: pages, then 3 objects per page: page, image, content
	pageIDs := make([]string, len(imgs))
	for i := range imgs {
		pageIDs[i] = fmt.Sprintf("%d 0 R", 3+i*3)
	}
	obj(func() { buf.WriteString("<< /Type /Catalog /Pages 2 0 R >>") })
	obj(func() {
		fmt.Fprintf(&buf, "<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(pageIDs, " "), len(imgs))
	})
	for i, im := range imgs {
		pw := float64(im.w) * 72 / 96
		ph := float64(im.h) * 72 / 96
		imgID, contentID := 4+i*3, 5+i*3
		obj(func() {
			fmt.Fprintf(&buf, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] "+
				"/Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>", pw, ph, imgID, contentID)
		})
		obj(func() {
			fmt.Fprintf(&buf, "<< /Type /XObject /Subtype /Image /Width %d /Height %d "+
				"/ColorSpace /%s /BitsPerComponent 8 /Filter /%s /Length %d >>\nstream\n",
				im.w, im.h, im.cs, im.filter, len(im.data))
			buf.Write(im.data)
			buf.WriteString("\nendstream")
		})
		content := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", pw, ph)
		obj(func() {
			fmt.Fprintf(&buf, "<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
		})
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)

	return os.WriteFile(out, buf.Bytes(), 0o644)
}

// ---------- PDF compression and PDF -> images (Ghostscript / Poppler) ----------

var errNoGhostscript = errors.New("Ghostscript is not installed. Install it with:  sudo apt install ghostscript")

func compressPDF(in, out string, lvl Level) error {
	gs, err := exec.LookPath("gs")
	if err != nil {
		return errNoGhostscript
	}
	setting := map[Level]string{Light: "/printer", Balanced: "/ebook", Strong: "/screen"}[lvl]
	cmd := exec.Command(gs,
		"-sDEVICE=pdfwrite", "-dCompatibilityLevel=1.5", "-dPDFSETTINGS="+setting,
		"-dNOPAUSE", "-dQUIET", "-dBATCH", "-dSAFER",
		"-dDetectDuplicateImages=true", "-dCompressFonts=true",
		"-sOutputFile="+out, in)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ghostscript failed: %s", firstLine(msg, err))
	}
	return nil
}

// pdfToImages renders every page at 150 DPI. A one-page PDF becomes
// name.png; a longer one becomes a folder name_pages/ with page-01.png ...
func pdfToImages(in string, t Target) ([]string, error) {
	base := strings.TrimSuffix(in, filepath.Ext(in))
	dir := uniquePath(base + "_pages")
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, err
	}
	ext := string(t)

	if tool, err := exec.LookPath("pdftoppm"); err == nil {
		flag := "-png"
		if t == ToJPG {
			flag = "-jpeg"
		}
		cmd := exec.Command(tool, flag, "-r", "150", in, filepath.Join(dir, "page"))
		if msg, err := cmd.CombinedOutput(); err != nil {
			os.RemoveAll(dir)
			return nil, fmt.Errorf("pdftoppm failed: %s", firstLine(msg, err))
		}
	} else if gs, err := exec.LookPath("gs"); err == nil {
		device := "png16m"
		if t == ToJPG {
			device = "jpeg"
		}
		cmd := exec.Command(gs, "-sDEVICE="+device, "-r150", "-dJPEGQ=90",
			"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4",
			"-dNOPAUSE", "-dQUIET", "-dBATCH", "-dSAFER",
			"-sOutputFile="+filepath.Join(dir, "page-%02d."+ext), in)
		if msg, err := cmd.CombinedOutput(); err != nil {
			os.RemoveAll(dir)
			return nil, fmt.Errorf("ghostscript failed: %s", firstLine(msg, err))
		}
	} else {
		os.RemoveAll(dir)
		return nil, errNoGhostscript
	}

	entries, _ := os.ReadDir(dir)
	var pages []string
	for _, e := range entries {
		pages = append(pages, filepath.Join(dir, e.Name()))
	}
	sort.Strings(pages)
	if len(pages) == 0 {
		os.RemoveAll(dir)
		return nil, errors.New("no pages were rendered")
	}

	// pdftoppm writes .jpg for -jpeg; normalise names to page-01.ext
	for i, p := range pages {
		want := filepath.Join(dir, fmt.Sprintf("page-%02d.%s", i+1, ext))
		if p != want {
			os.Rename(p, want)
			pages[i] = want
		}
	}

	if len(pages) == 1 {
		single := uniquePath(base + "." + ext)
		if err := os.Rename(pages[0], single); err == nil {
			os.Remove(dir)
			return []string{single}, nil
		}
	}
	return pages, nil
}

func firstLine(msg []byte, err error) string {
	s := strings.TrimSpace(string(msg))
	if s == "" {
		return err.Error()
	}
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	return s
}
