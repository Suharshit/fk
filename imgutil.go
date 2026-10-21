package main

import (
	"bufio"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"
	"sort"
)

// ---------- EXIF orientation ----------

// jpegOrientation reads the EXIF orientation tag (1-8) of a JPEG file.
// It returns 1 when there is no tag or it can't be read. Phones often save
// photos sideways and rely on this tag, and re-encoding drops it, so we
// rotate the pixels instead.
func jpegOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()
	r := bufio.NewReader(f)

	var soi [2]byte
	if _, err := io.ReadFull(r, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return 1
	}
	for {
		var m [4]byte
		if _, err := io.ReadFull(r, m[:]); err != nil || m[0] != 0xFF {
			return 1
		}
		marker := m[1]
		size := int(binary.BigEndian.Uint16(m[2:])) - 2
		if size < 0 || marker == 0xDA { // start of scan: no more metadata
			return 1
		}
		seg := make([]byte, size)
		if _, err := io.ReadFull(r, seg); err != nil {
			return 1
		}
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return orientationFromTIFF(seg[6:])
		}
	}
}

func orientationFromTIFF(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for i := 0; i < n; i++ {
		e := off + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			v := int(bo.Uint16(t[e+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation turns the pixels so the image displays upright.
func applyOrientation(img image.Image, o int) image.Image {
	src := toNRGBA(img)
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-sx, sy
			case 3:
				dx, dy = w-1-sx, h-1-sy
			case 4:
				dx, dy = sx, h-1-sy
			case 5:
				dx, dy = sy, sx
			case 6:
				dx, dy = h-1-sy, sx
			case 7:
				dx, dy = h-1-sy, w-1-sx
			case 8:
				dx, dy = sy, w-1-sx
			default:
				dx, dy = sx, sy
			}
			si := sy*src.Stride + sx*4
			di := dy*dst.Stride + dx*4
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// ---------- colour quantization (lossy PNG / GIF) ----------

// quantize reduces img to at most n colours using median cut, then maps the
// pixels to that palette with Floyd-Steinberg dithering. Images that
// already use n colours or fewer keep their exact colours.
func quantize(img image.Image, n int) *image.Paletted {
	src := toNRGBA(img)
	w, h := src.Bounds().Dx(), src.Bounds().Dy()

	if pal, ok := exactPalette(src, n); ok {
		dst := image.NewPaletted(src.Bounds(), pal)
		idx := make(map[color.NRGBA]uint8, len(pal))
		for i, c := range pal {
			idx[c.(color.NRGBA)] = uint8(i)
		}
		for i, j := 0, 0; i < len(src.Pix); i, j = i+4, j+1 {
			c := color.NRGBA{src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3]}
			dst.Pix[j] = idx[c]
		}
		return dst
	}

	pal := medianCut(src, n)
	dst := image.NewPaletted(src.Bounds(), pal)

	// nearest-colour cache keyed on reduced precision RGBA
	cache := make([]int16, 1<<19)
	for i := range cache {
		cache[i] = -1
	}
	nearest := func(r, g, b, a int32) uint8 {
		key := (r>>3)<<14 | (g>>3)<<9 | (b>>3)<<4 | a>>4
		if v := cache[key]; v >= 0 {
			return uint8(v)
		}
		best, bestD := 0, int32(1<<30)
		for i, pc := range pal {
			c := pc.(color.NRGBA)
			dr, dg, db, da := r-int32(c.R), g-int32(c.G), b-int32(c.B), a-int32(c.A)
			d := dr*dr + dg*dg + db*db + 2*da*da
			if d < bestD {
				best, bestD = i, d
			}
		}
		cache[key] = int16(best)
		return uint8(best)
	}

	// Floyd-Steinberg error diffusion, two rows of error buffers
	cur := make([][4]float32, w+2)
	next := make([][4]float32, w+2)
	clamp := func(v float32) int32 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return int32(v + 0.5)
	}
	for y := 0; y < h; y++ {
		for i := range next {
			next[i] = [4]float32{}
		}
		for x := 0; x < w; x++ {
			si := y*src.Stride + x*4
			var want [4]float32
			for c := 0; c < 4; c++ {
				want[c] = float32(src.Pix[si+c]) + cur[x+1][c]
			}
			r, g, b, a := clamp(want[0]), clamp(want[1]), clamp(want[2]), clamp(want[3])
			pi := nearest(r, g, b, a)
			dst.Pix[y*dst.Stride+x] = pi
			got := pal[pi].(color.NRGBA)
			gv := [4]float32{float32(got.R), float32(got.G), float32(got.B), float32(got.A)}
			for c := 0; c < 4; c++ {
				e := want[c] - gv[c]
				cur[x+2][c] += e * 7 / 16
				next[x][c] += e * 3 / 16
				next[x+1][c] += e * 5 / 16
				next[x+2][c] += e * 1 / 16
			}
		}
		cur, next = next, cur
	}
	return dst
}

func exactPalette(src *image.NRGBA, n int) (color.Palette, bool) {
	seen := make(map[color.NRGBA]struct{}, n+1)
	for i := 0; i < len(src.Pix); i += 4 {
		c := color.NRGBA{src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3]}
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			if len(seen) > n {
				return nil, false
			}
		}
	}
	pal := make(color.Palette, 0, len(seen))
	for c := range seen {
		pal = append(pal, c)
	}
	return pal, true
}

type box struct{ px [][4]uint8 }

func (b *box) widest() (ch int, span int) {
	var lo, hi [4]uint8
	lo = [4]uint8{255, 255, 255, 255}
	for _, p := range b.px {
		for c := 0; c < 4; c++ {
			if p[c] < lo[c] {
				lo[c] = p[c]
			}
			if p[c] > hi[c] {
				hi[c] = p[c]
			}
		}
	}
	for c := 0; c < 4; c++ {
		if s := int(hi[c]) - int(lo[c]); s > span {
			ch, span = c, s
		}
	}
	return
}

func medianCut(src *image.NRGBA, n int) color.Palette {
	// sample at most ~250k pixels so big photos stay fast
	total := len(src.Pix) / 4
	step := total/250000 + 1
	px := make([][4]uint8, 0, total/step+1)
	for i := 0; i < total; i += step {
		j := i * 4
		px = append(px, [4]uint8{src.Pix[j], src.Pix[j+1], src.Pix[j+2], src.Pix[j+3]})
	}

	boxes := []*box{{px: px}}
	for len(boxes) < n {
		// split the box with the largest (span x population)
		bi, bestScore, bestCh := -1, 0, 0
		for i, b := range boxes {
			if len(b.px) < 2 {
				continue
			}
			ch, span := b.widest()
			if score := span * len(b.px); span > 0 && score > bestScore {
				bi, bestScore, bestCh = i, score, ch
			}
		}
		if bi < 0 {
			break
		}
		b := boxes[bi]
		ch := bestCh
		sort.Slice(b.px, func(i, j int) bool { return b.px[i][ch] < b.px[j][ch] })
		mid := len(b.px) / 2
		left := &box{px: b.px[:mid]}
		right := &box{px: b.px[mid:]}
		boxes[bi] = left
		boxes = append(boxes, right)
	}

	pal := make(color.Palette, 0, len(boxes))
	for _, b := range boxes {
		var sum [4]int
		for _, p := range b.px {
			for c := 0; c < 4; c++ {
				sum[c] += int(p[c])
			}
		}
		k := len(b.px)
		if k == 0 {
			continue
		}
		pal = append(pal, color.NRGBA{uint8(sum[0] / k), uint8(sum[1] / k), uint8(sum[2] / k), uint8(sum[3] / k)})
	}
	return pal
}
