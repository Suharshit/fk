package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testAPI(t *testing.T, mutate func(*apiConfig)) *httptest.Server {
	t.Helper()
	cfg := defaultAPIConfig()
	cfg.workDir = t.TempDir()
	cfg.jobs = 2
	if mutate != nil {
		mutate(&cfg)
	}
	srv := httptest.NewServer(newAPIHandler(cfg))
	t.Cleanup(srv.Close)
	return srv
}

type part struct {
	field, name string
	data        []byte
}

func post(t *testing.T, url string, fields map[string]string, parts ...part) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	for _, p := range parts {
		w, _ := mw.CreateFormFile(p.field, p.name)
		w.Write(p.data)
	}
	mw.Close()
	res, err := http.Post(url, mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func jpegBytes(t *testing.T, w, h, q int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, photo(w, h), &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.png")
	writePNG(t, p, photo(w, h))
	b, _ := os.ReadFile(p)
	return b
}

func errorOf(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	var e struct{ Error string }
	json.NewDecoder(res.Body).Decode(&e)
	return e.Error
}

func TestAPIHealthAndVersion(t *testing.T) {
	srv := testAPI(t, nil)

	res, _ := http.Get(srv.URL + "/healthz")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || strings.TrimSpace(string(body)) != "ok" {
		t.Fatalf("healthz: %d %q", res.StatusCode, body)
	}

	res, _ = http.Get(srv.URL + "/version")
	var v map[string]any
	json.NewDecoder(res.Body).Decode(&v)
	if v["version"] == nil || v["max_upload_mb"] == nil {
		t.Fatalf("version: %v", v)
	}

	res, _ = http.Get(srv.URL + "/")
	body, _ = io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Compress &amp; convert") || res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("home page or security headers missing")
	}
}

func TestAPICompress(t *testing.T) {
	srv := testAPI(t, nil)
	in := jpegBytes(t, 2400, 1600, 98)

	res := post(t, srv.URL+"/api/compress", map[string]string{"level": "strong"}, part{"file", "Holiday Photo.JPG", in})
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("status %d: %s", res.StatusCode, out)
	}
	if res.Header.Get("Content-Type") != "image/jpeg" || len(out) >= len(in) {
		t.Fatalf("expected a smaller JPEG, got %s %d >= %d", res.Header.Get("Content-Type"), len(out), len(in))
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "Holiday_Photo_compressed.jpg") {
		t.Fatalf("download name: %s", cd)
	}

	if res := post(t, srv.URL+"/api/compress", map[string]string{"level": "extreme"}, part{"file", "a.jpg", in}); res.StatusCode != 400 {
		t.Fatalf("bad level: %d", res.StatusCode)
	}
}

func TestAPIConvertAndMerge(t *testing.T) {
	srv := testAPI(t, nil)
	png1 := pngBytes(t, 300, 200)

	res := post(t, srv.URL+"/api/convert", map[string]string{"to": "jpeg"}, part{"file", "shot.png", png1})
	out, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || !bytes.HasPrefix(out, []byte{0xFF, 0xD8}) {
		t.Fatalf("png->jpg: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	if res := post(t, srv.URL+"/api/convert", map[string]string{"to": "png"}, part{"file", "shot.png", png1}); res.StatusCode != 422 {
		t.Fatalf("png->png should be 422, got %d", res.StatusCode)
	}

	res = post(t, srv.URL+"/api/merge", nil,
		part{"file", "a.jpg", jpegBytes(t, 200, 100, 80)}, part{"file", "b.png", png1})
	out, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.HasPrefix(out, []byte("%PDF-")) || !bytes.Contains(out, []byte("/Count 2")) {
		t.Fatalf("merge: %d", res.StatusCode)
	}

	// multi-page PDF -> zip of images (needs pdftoppm or ghostscript)
	_, e1 := exec.LookPath("pdftoppm")
	_, e2 := exec.LookPath("gs")
	if e1 != nil && e2 != nil {
		t.Skip("no PDF renderer installed")
	}
	res = post(t, srv.URL+"/api/convert", map[string]string{"to": "png"}, part{"file", "doc.pdf", out})
	zipped, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("X-Fk-Pages") != "2" {
		t.Fatalf("pdf->png: %d pages=%s", res.StatusCode, res.Header.Get("X-Fk-Pages"))
	}
	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil || len(zr.File) != 2 {
		t.Fatalf("zip: %v, %d files", err, len(zr.File))
	}
}

func TestAPIRejectsBadInput(t *testing.T) {
	srv := testAPI(t, func(c *apiConfig) {
		c.maxBytes = 200 << 10 // 200 KB
		c.maxPixels = 1_000_000
	})

	cases := []struct {
		name   string
		parts  []part
		status int
	}{
		{"wrong type", []part{{"file", "notes.txt", []byte("hello")}}, 415},
		{"fake png", []part{{"file", "fake.png", []byte("not really a png")}}, 422},
		{"fake pdf", []part{{"file", "fake.pdf", []byte("hello")}}, 422},
		{"too many pixels", []part{{"file", "big.jpg", jpegBytes(t, 1500, 1000, 10)}}, 422},
		{"too large", []part{{"file", "huge.png", bytes.Repeat([]byte{1}, 300<<10)}}, 413},
		{"no file", nil, 400},
	}
	for _, c := range cases {
		res := post(t, srv.URL+"/api/compress", nil, c.parts...)
		msg := errorOf(t, res)
		if res.StatusCode != c.status || msg == "" {
			t.Errorf("%s: got %d %q, want %d", c.name, res.StatusCode, msg, c.status)
		}
	}

	// GET on an API route is not allowed
	res, _ := http.Get(srv.URL + "/api/compress")
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/compress: %d", res.StatusCode)
	}
}

func TestAPILeavesNoFilesBehind(t *testing.T) {
	work := t.TempDir()
	srv := testAPI(t, func(c *apiConfig) { c.workDir = work })
	post(t, srv.URL+"/api/compress", nil, part{"file", "a.jpg", jpegBytes(t, 400, 300, 95)}).Body.Close()
	post(t, srv.URL+"/api/compress", nil, part{"file", "x.txt", []byte("x")}).Body.Close()
	entries, _ := os.ReadDir(work)
	if len(entries) != 0 {
		t.Fatalf("temp folders left behind: %d", len(entries))
	}
}

func TestSafeName(t *testing.T) {
	for in, want := range map[string]string{
		"../../etc/passwd.png":            "passwd.png",
		`C:\Users\me\scan.PDF`:            "scan.pdf",
		"my photo (1).JPG":                "my_photo_1.jpg",
		"....png":                         "upload.png",
		"résumé final.pdf":                "rsum_final.pdf",
		strings.Repeat("a", 200) + ".jpg": strings.Repeat("a", 80) + ".jpg",
	} {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}
