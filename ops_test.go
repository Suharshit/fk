package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// photo makes a smooth gradient with a red block in the top-left corner.
func photo(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), uint8((x + y) % 256), 255})
		}
	}
	for y := 0; y < h/5; y++ {
		for x := 0; x < w/5; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	return img
}

func writeJPEG(t *testing.T, path string, img image.Image, q int) {
	t.Helper()
	f, _ := os.Create(path)
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, _ := os.Create(path)
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// withOrientation inserts an EXIF APP1 segment with the given orientation.
func withOrientation(jpg []byte, o uint16) []byte {
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, // header, IFD at 8
		0, 1, // one entry
		0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(o >> 8), byte(o), 0, 0,
		0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	seg = append(seg, payload...)
	return append(append([]byte{0xFF, 0xD8}, seg...), jpg[2:]...)
}

func decode(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("%s does not decode: %v", path, err)
	}
	return img
}

func TestCompressJPEG(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "photo.jpg")
	writeJPEG(t, in, photo(3000, 2000), 98)

	for _, lvl := range []Level{Light, Balanced, Strong} {
		r := compressFile(in, lvl)
		if r.Err != nil {
			t.Fatalf("%s: %v", lvl, r.Err)
		}
		if r.Out == "" || r.OutSize >= r.InSize {
			t.Fatalf("%s: expected a smaller file, got %+v", lvl, r)
		}
		b := decode(t, r.Out).Bounds()
		if lvl == Strong && (b.Dx() != 1600 || b.Dy() != 1066) {
			t.Fatalf("strong should resize to 1600x1066, got %v", b)
		}
	}
	// outputs never overwrite each other
	if _, err := os.Stat(filepath.Join(dir, "photo_compressed-3.jpg")); err != nil {
		t.Fatal("expected photo_compressed-3.jpg to exist")
	}
}

func TestCompressPNGKeepsAlpha(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 800, 600))
	src := photo(800, 600)
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			c := src.RGBAAt(x, y)
			c.A = 255
			if x < 100 {
				c = color.RGBA{}
			}
			img.Set(x, y, color.NRGBA(c))
		}
	}
	in := filepath.Join(dir, "logo.png")
	writePNG(t, in, img)

	// smooth gradients don't shrink with fewer colours: fk must keep the
	// original rather than write a bigger file
	r := compressFile(in, Balanced)
	if r.Err != nil || (r.Out != "" && r.OutSize >= r.InSize) {
		t.Fatalf("compress made the file bigger: %+v", r)
	}
	out := quantize(decode(t, in), 256)
	if _, _, _, a := out.At(10, 10).RGBA(); a != 0 {
		t.Fatalf("transparent pixel became opaque (alpha %d)", a)
	}
	if _, _, _, a := out.At(500, 300).RGBA(); a == 0 {
		t.Fatal("opaque pixel became transparent")
	}
}

func TestEXIFOrientationIsApplied(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	jpeg.Encode(&buf, photo(400, 200), &jpeg.Options{Quality: 95})
	in := filepath.Join(dir, "phone.jpg")
	os.WriteFile(in, withOrientation(buf.Bytes(), 6), 0o644)

	if o := jpegOrientation(in); o != 6 {
		t.Fatalf("orientation = %d, want 6", o)
	}
	r := convertFile(in, ToPNG)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	out := decode(t, r.Out)
	if b := out.Bounds(); b.Dx() != 200 || b.Dy() != 400 {
		t.Fatalf("rotated size = %v, want 200x400", b)
	}
	// orientation 6 = rotate 90° clockwise: the red top-left block moves to the top-right
	if r, g, _, _ := out.At(190, 10).RGBA(); r>>8 < 200 || g>>8 > 60 {
		t.Fatalf("expected red at top-right after rotation, got r=%d g=%d", r>>8, g>>8)
	}
}

func TestConvertImageFormats(t *testing.T) {
	dir := t.TempDir()
	png1 := filepath.Join(dir, "shot.png")
	img := image.NewNRGBA(image.Rect(0, 0, 300, 200))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	img.Set(0, 0, color.NRGBA{0, 0, 0, 0}) // a transparent pixel
	writePNG(t, png1, img)

	for _, target := range []Target{ToJPG, ToGIF, ToBMP, ToTIFF} {
		r := convertFile(png1, target)
		if r.Err != nil {
			t.Fatalf("%s: %v", target, r.Err)
		}
		if !strings.HasSuffix(r.Out, "."+string(target)) {
			t.Fatalf("%s: unexpected output name %s", target, r.Out)
		}
		decode(t, r.Out)
	}
	// JPG: transparent pixel should become white, not black
	if r, _, _, _ := decode(t, filepath.Join(dir, "shot.jpg")).At(0, 0).RGBA(); r>>8 < 200 {
		t.Fatalf("transparent pixel became dark in JPG (r=%d)", r>>8)
	}
	// same format is skipped
	if r := convertFile(png1, ToPNG); !r.Discarded {
		t.Fatal("PNG -> PNG should be skipped")
	}
}

func TestImagesToPDFAndBack(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jpg")
	b := filepath.Join(dir, "b.png")
	writeJPEG(t, a, photo(640, 480), 90)
	writePNG(t, b, photo(320, 640))

	r := mergeToPDF([]string{a, b})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	data, _ := os.ReadFile(r.Out)
	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) || !bytes.Contains(data, []byte("/Count 2")) {
		t.Fatal("merged PDF looks wrong")
	}

	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("ghostscript not installed; skipping PDF render checks")
	}
	// the PDF must be valid enough for Ghostscript to render both pages
	conv := convertFile(r.Out, ToPNG)
	if conv.Err != nil {
		t.Fatal(conv.Err)
	}
	if len(conv.Extra) != 1 {
		t.Fatalf("expected 2 pages, got %d", len(conv.Extra)+1)
	}
	p1 := decode(t, conv.Out).Bounds()
	if p1.Dx() < 900 || p1.Dx() > 1100 { // 640px at 96dpi rendered at 150dpi = 1000px
		t.Fatalf("page 1 rendered at unexpected width %d", p1.Dx())
	}

	// single page PDF -> single image next to the PDF
	single := filepath.Join(dir, "single.pdf")
	if err := imagesToPDF(single, []string{a}); err != nil {
		t.Fatal(err)
	}
	s := convertFile(single, ToJPG)
	if s.Err != nil || filepath.Base(s.Out) != "single.jpg" || len(s.Extra) != 0 {
		t.Fatalf("single page conversion: %+v", s)
	}

	// PDF compression runs (result may be discarded if not smaller)
	c := compressFile(r.Out, Strong)
	if c.Err != nil {
		t.Fatal(c.Err)
	}
}

func TestCommonTargets(t *testing.T) {
	// PDF -> PDF is skipped per file, so PDF stays on offer for the JPG
	got := commonTargets([]Kind{KindPDF, KindJPEG})
	if len(got) != 3 || got[0] != ToPNG || got[1] != ToJPG || got[2] != ToPDF {
		t.Fatalf("pdf+jpg targets = %v", got)
	}
	got = commonTargets([]Kind{KindPNG})
	if contains(got, ToPNG) || !contains(got, ToPDF) {
		t.Fatalf("png targets = %v", got)
	}
}

func TestTruncateANSI(t *testing.T) {
	s := truncateANSI("\x1b[1mhello\x1b[0m world", 7)
	if s != "\x1b[1mhello\x1b[0m w" {
		t.Fatalf("got %q", s)
	}
}
