package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/webp" // decode only
)

// Kind is the family a file belongs to.
type Kind int

const (
	KindOther Kind = iota
	KindPDF
	KindPNG
	KindJPEG
)

func kindOf(name string) Kind {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return KindPDF
	case ".png":
		return KindPNG
	case ".jpg", ".jpeg":
		return KindJPEG
	}
	return KindOther
}

func (k Kind) Label() string {
	switch k {
	case KindPDF:
		return "PDF"
	case KindPNG:
		return "PNG"
	case KindJPEG:
		return "JPG"
	}
	return "?"
}

func (k Kind) isImage() bool { return k == KindPNG || k == KindJPEG }

// Level is a compression strength.
type Level int

const (
	Light Level = iota
	Balanced
	Strong
)

func (l Level) String() string {
	return [...]string{"light", "balanced", "strong"}[l]
}

// Result describes what happened to one input file.
type Result struct {
	In, Out   string
	InSize    int64
	OutSize   int64
	Extra     []string // extra output files (multi-page PDF -> images)
	Note      string
	Err       error
	Discarded bool // output was bigger than the input and was removed
}

// ---------- compression ----------

func compressFile(path string, lvl Level) Result {
	r := Result{In: path, InSize: fileSize(path)}
	k := kindOf(path)
	ext := filepath.Ext(path)
	out := uniquePath(strings.TrimSuffix(path, ext) + "_compressed" + strings.ToLower(ext))

	var err error
	switch {
	case k == KindPDF:
		err = compressPDF(path, out, lvl)
	case k.isImage():
		err = compressImage(path, out, k, lvl)
	default:
		err = fmt.Errorf("unsupported file type %q", ext)
	}
	if err != nil {
		os.Remove(out)
		r.Err = err
		return r
	}
	r.Out = out
	r.OutSize = fileSize(out)
	if r.OutSize >= r.InSize {
		os.Remove(out)
		r.Out = ""
		r.Discarded = true
		r.Note = "already well compressed, original kept"
	}
	return r
}

func compressImage(in, out string, k Kind, lvl Level) error {
	img, err := loadImage(in)
	if err != nil {
		return err
	}
	switch {
	case lvl == Strong && k == KindJPEG:
		img = fitWithin(img, 1600) // photos: plenty for screens
	case lvl != Light:
		img = fitWithin(img, 2560) // PNGs are often screenshots: keep text readable
	}

	if k == KindJPEG {
		q := map[Level]int{Light: 85, Balanced: 72, Strong: 55}[lvl]
		return saveJPEG(out, img, q)
	}

	// PNG
	if lvl == Light {
		return savePNG(out, img, true) // lossless: just re-encode at best compression
	}
	colors := map[Level]int{Balanced: 256, Strong: 64}[lvl]
	return savePNG(out, quantize(img, colors), true)
}

// ---------- conversion ----------

// Target is an output format for conversion.
type Target string

const (
	ToPNG  Target = "png"
	ToJPG  Target = "jpg"
	ToPDF  Target = "pdf"
	ToGIF  Target = "gif"
	ToBMP  Target = "bmp"
	ToTIFF Target = "tiff"
)

func (t Target) Label() string {
	switch t {
	case ToPNG:
		return "PNG  (lossless, keeps transparency)"
	case ToJPG:
		return "JPG  (small, best for photos)"
	case ToPDF:
		return "PDF  (one PDF per image)"
	case ToGIF:
		return "GIF  (256 colours)"
	case ToBMP:
		return "BMP  (uncompressed)"
	case ToTIFF:
		return "TIFF (print / archival)"
	}
	return string(t)
}

func targetsFor(k Kind) []Target {
	switch k {
	case KindPDF:
		return []Target{ToPNG, ToJPG}
	case KindPNG:
		return []Target{ToJPG, ToPDF, ToGIF, ToBMP, ToTIFF}
	case KindJPEG:
		return []Target{ToPNG, ToPDF, ToGIF, ToBMP, ToTIFF}
	}
	return nil
}

// commonTargets returns the targets that make sense for every selected file
// (ignoring "convert to the same format", which is skipped per file).
func commonTargets(kinds []Kind) []Target {
	all := []Target{ToPNG, ToJPG, ToPDF, ToGIF, ToBMP, ToTIFF}
	var out []Target
	for _, t := range all {
		ok, useful := true, false
		for _, k := range kinds {
			if sameFormat(k, t) {
				continue
			}
			useful = true
			if !contains(targetsFor(k), t) {
				ok = false
				break
			}
		}
		if ok && useful {
			out = append(out, t)
		}
	}
	return out
}

func sameFormat(k Kind, t Target) bool {
	return (k == KindPNG && t == ToPNG) || (k == KindJPEG && t == ToJPG) || (k == KindPDF && t == ToPDF)
}

func convertFile(path string, t Target) Result {
	r := Result{In: path, InSize: fileSize(path)}
	k := kindOf(path)
	if sameFormat(k, t) {
		r.Note = "already " + strings.ToUpper(string(t)) + ", skipped"
		r.Discarded = true
		return r
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))

	if k == KindPDF {
		outs, err := pdfToImages(path, t)
		if err != nil {
			r.Err = err
			return r
		}
		r.Out = outs[0]
		r.Extra = outs[1:]
		for _, o := range outs {
			r.OutSize += fileSize(o)
		}
		return r
	}

	out := uniquePath(base + "." + string(t))
	img, err := loadImage(path)
	if err == nil {
		switch t {
		case ToPNG:
			err = savePNG(out, img)
		case ToJPG:
			err = saveJPEG(out, img, 90)
		case ToPDF:
			err = imagesToPDF(out, []string{path})
		case ToGIF:
			err = saveWith(out, func(f *os.File) error {
				return gif.Encode(f, quantize(img, 256), nil)
			})
		case ToBMP:
			err = saveWith(out, func(f *os.File) error { return bmp.Encode(f, img) })
		case ToTIFF:
			err = saveWith(out, func(f *os.File) error {
				return tiff.Encode(f, img, &tiff.Options{Compression: tiff.Deflate})
			})
		default:
			err = fmt.Errorf("cannot convert to %s", t)
		}
	}
	if err != nil {
		os.Remove(out)
		r.Err = err
		return r
	}
	r.Out = out
	r.OutSize = fileSize(out)
	return r
}

// mergeToPDF puts every image into one PDF, one image per page.
func mergeToPDF(paths []string) Result {
	dir := filepath.Dir(paths[0])
	out := uniquePath(filepath.Join(dir, "merged.pdf"))
	r := Result{In: fmt.Sprintf("%d images", len(paths)), Out: out}
	for _, p := range paths {
		r.InSize += fileSize(p)
	}
	if err := imagesToPDF(out, paths); err != nil {
		os.Remove(out)
		r.Out = ""
		r.Err = err
		return r
	}
	r.OutSize = fileSize(out)
	return r
}

// ---------- image helpers ----------

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read image: %w", err)
	}
	if kindOf(path) == KindJPEG {
		if o := jpegOrientation(path); o > 1 {
			img = applyOrientation(img, o)
		}
	}
	return img, nil
}

func saveWith(path string, write func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func saveJPEG(path string, img image.Image, quality int) error {
	return saveWith(path, func(f *os.File) error {
		return jpeg.Encode(f, flatten(img), &jpeg.Options{Quality: quality})
	})
}

// savePNG writes a PNG. best=true squeezes harder but is much slower on
// big images, so it's only used when compressing.
func savePNG(path string, img image.Image, best ...bool) error {
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if len(best) > 0 && best[0] {
		enc.CompressionLevel = png.BestCompression
	}
	return saveWith(path, func(f *os.File) error { return enc.Encode(f, img) })
}

// flatten draws img on a white background, so transparent areas don't turn
// black when saved as JPG.
func flatten(img image.Image) image.Image {
	if !hasAlpha(img) {
		return img
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, &image.Uniform{color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, b, img, b.Min, draw.Over)
	return dst
}

func hasAlpha(img image.Image) bool {
	switch m := img.(type) {
	case *image.YCbCr, *image.Gray, *image.Gray16, *image.CMYK:
		return false
	case *image.RGBA:
		return !m.Opaque()
	case *image.NRGBA:
		return !m.Opaque()
	case *image.Paletted:
		return !m.Opaque()
	}
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return !o.Opaque()
	}
	return true
}

// fitWithin shrinks img so its longest side is at most max pixels.
func fitWithin(img image.Image, max int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return img
	}
	if w >= h {
		h = h * max / w
		w = max
	} else {
		w = w * max / h
		h = max
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

// ---------- file helpers ----------

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

// uniquePath returns path, or path with -2, -3 ... added before the
// extension, so existing files are never overwritten.
func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		p := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return p
		}
	}
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
