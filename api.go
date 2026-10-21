package main

// fk api: a small web app + HTTP API for running fk on a server.
//
// Users upload a file, pick compress / convert / merge, and get the result
// back as a download. Nothing is kept: every request works in its own
// temporary folder, which is deleted as soon as the response is sent.

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type apiConfig struct {
	addr      string
	port      int
	maxBytes  int64  // largest request body
	workDir   string // where per-request temp folders go
	jobs      int    // how many files are processed at the same time
	maxPixels int    // largest image accepted (width x height)
	maxPages  int    // most PDF pages rendered to images
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func envStr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func defaultAPIConfig() apiConfig {
	return apiConfig{
		addr:      envStr("FK_ADDR", "0.0.0.0"),
		port:      envInt("PORT", 8000),
		maxBytes:  int64(envInt("FK_MAX_UPLOAD_MB", 25)) << 20,
		workDir:   envStr("FK_WORKDIR", os.TempDir()),
		jobs:      envInt("FK_MAX_JOBS", runtime.NumCPU()),
		maxPixels: envInt("FK_MAX_MEGAPIXELS", 60) * 1_000_000,
		maxPages:  envInt("FK_MAX_PDF_PAGES", 50),
	}
}

func runAPI(args []string) error {
	cfg := defaultAPIConfig()
	fset := flag.NewFlagSet("api", flag.ContinueOnError)
	fset.StringVar(&cfg.addr, "addr", cfg.addr, "address to listen on")
	fset.IntVar(&cfg.port, "port", cfg.port, "port to listen on")
	fset.StringVar(&cfg.workDir, "workdir", cfg.workDir, "folder for temporary files")
	if err := fset.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.workDir, 0o750); err != nil {
		return fmt.Errorf("workdir: %w", err)
	}
	maxPDFPages = cfg.maxPages
	toolTimeout = 2 * time.Minute

	addr := net.JoinHostPort(cfg.addr, strconv.Itoa(cfg.port))
	srv := &http.Server{
		Addr:              addr,
		Handler:           newAPIHandler(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	_, gsErr := exec.LookPath("gs")
	log.Printf("fk api %s listening on %s (max upload %d MB, %d parallel jobs, ghostscript=%v)",
		version, addr, cfg.maxBytes>>20, cfg.jobs, gsErr == nil)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Printf("shutting down: finishing requests in progress")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

type api struct {
	cfg apiConfig
	sem chan struct{}
}

func newAPIHandler(cfg apiConfig) http.Handler {
	if cfg.jobs < 1 {
		cfg.jobs = 1
	}
	a := &api{cfg: cfg, sem: make(chan struct{}, cfg.jobs)}
	static, _ := fs.Sub(webFiles, "web")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		page, _ := fs.ReadFile(static, "app.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		_, gsErr := exec.LookPath("gs")
		host, _ := os.Hostname()
		writeJSON(w, http.StatusOK, map[string]any{
			"version":       version,
			"host":          host,
			"ghostscript":   gsErr == nil,
			"max_upload_mb": cfg.maxBytes >> 20,
		})
	})
	mux.HandleFunc("POST /api/compress", a.compress)
	mux.HandleFunc("POST /api/convert", a.convert)
	mux.HandleFunc("POST /api/merge", a.merge)

	return logRequests(securityHeaders(mux))
}

// ---------- handlers ----------

func (a *api) compress(w http.ResponseWriter, r *http.Request) {
	a.withJob(w, r, func(job *upload) {
		if len(job.files) != 1 {
			writeErr(w, http.StatusBadRequest, "send exactly one file in the \"file\" field")
			return
		}
		lvl, ok := map[string]Level{"": Balanced, "light": Light, "balanced": Balanced, "strong": Strong}[strings.ToLower(job.fields["level"])]
		if !ok {
			writeErr(w, http.StatusBadRequest, "level must be light, balanced or strong")
			return
		}
		res := compressFile(job.files[0], lvl)
		switch {
		case res.Err != nil:
			writeOpErr(w, res.Err)
		case res.Discarded:
			w.Header().Set("X-Fk-Note", res.Note)
			sendFile(w, job.files[0], filepath.Base(job.files[0]), res.InSize)
		default:
			sendFile(w, res.Out, filepath.Base(res.Out), res.InSize)
		}
	})
}

func (a *api) convert(w http.ResponseWriter, r *http.Request) {
	a.withJob(w, r, func(job *upload) {
		if len(job.files) != 1 {
			writeErr(w, http.StatusBadRequest, "send exactly one file in the \"file\" field")
			return
		}
		in := job.files[0]
		to := strings.ToLower(job.fields["to"])
		if to == "jpeg" {
			to = "jpg"
		}
		t := Target(to)
		k := kindOf(in)
		if sameFormat(k, t) {
			writeErr(w, http.StatusUnprocessableEntity, "the file is already "+strings.ToUpper(to))
			return
		}
		if !contains(targetsFor(k), t) {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("a %s can be converted to: %s", k.Label(), targetNames(targetsFor(k))))
			return
		}
		res := convertFile(in, t)
		if res.Err != nil {
			writeOpErr(w, res.Err)
			return
		}
		if len(res.Extra) == 0 {
			sendFile(w, res.Out, filepath.Base(res.Out), res.InSize)
			return
		}
		// several pages: send them as one zip
		pages := append([]string{res.Out}, res.Extra...)
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		zipPath := filepath.Join(job.dir, base+"_pages.zip")
		if err := zipFiles(zipPath, pages); err != nil {
			writeErr(w, http.StatusInternalServerError, "could not build the zip")
			return
		}
		w.Header().Set("X-Fk-Pages", strconv.Itoa(len(pages)))
		sendFile(w, zipPath, filepath.Base(zipPath), res.InSize)
	})
}

func (a *api) merge(w http.ResponseWriter, r *http.Request) {
	a.withJob(w, r, func(job *upload) {
		if len(job.files) < 2 {
			writeErr(w, http.StatusBadRequest, "send at least two images in \"file\" fields")
			return
		}
		for _, f := range job.files {
			if !kindOf(f).isImage() {
				writeErr(w, http.StatusUnsupportedMediaType, "merge accepts PNG and JPG images only")
				return
			}
		}
		res := mergeToPDF(job.files)
		if res.Err != nil {
			writeOpErr(w, res.Err)
			return
		}
		sendFile(w, res.Out, "merged.pdf", res.InSize)
	})
}

// ---------- shared request handling ----------

type upload struct {
	dir    string
	files  []string
	fields map[string]string
}

const maxFilesPerRequest = 20

// withJob saves the upload into a fresh temp folder, waits for a free
// worker slot, runs fn, and always deletes the folder afterwards.
func (a *api) withJob(w http.ResponseWriter, r *http.Request, fn func(*upload)) {
	dir, err := os.MkdirTemp(a.cfg.workDir, "job-")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "server cannot create temp files")
		return
	}
	defer os.RemoveAll(dir)

	job, status, msg := a.readUpload(w, r, dir)
	if status != 0 {
		writeErr(w, status, msg)
		return
	}

	wait, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	select {
	case a.sem <- struct{}{}:
		defer func() { <-a.sem }()
	case <-wait.Done():
		w.Header().Set("Retry-After", "10")
		writeErr(w, http.StatusServiceUnavailable, "server is busy, try again in a few seconds")
		return
	}
	fn(job)
}

func (a *api) readUpload(w http.ResponseWriter, r *http.Request, dir string) (*upload, int, string) {
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.maxBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, http.StatusBadRequest, "send the file as multipart/form-data"
	}
	job := &upload{dir: dir, fields: map[string]string{}}
	tooBig := fmt.Sprintf("upload is larger than %d MB", a.cfg.maxBytes>>20)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, http.StatusRequestEntityTooLarge, tooBig
		}
		if err != nil {
			return nil, http.StatusBadRequest, "could not read the upload"
		}

		if part.FileName() == "" { // a normal form field
			v, _ := io.ReadAll(io.LimitReader(part, 256))
			job.fields[part.FormName()] = strings.TrimSpace(string(v))
			continue
		}

		if len(job.files) >= maxFilesPerRequest {
			return nil, http.StatusBadRequest, fmt.Sprintf("at most %d files per request", maxFilesPerRequest)
		}
		name := safeName(part.FileName())
		if kindOf(name) == KindOther {
			return nil, http.StatusUnsupportedMediaType, "only PDF, PNG and JPG files are supported"
		}
		dst := uniquePath(filepath.Join(dir, name))
		f, err := os.Create(dst)
		if err != nil {
			return nil, http.StatusInternalServerError, "server cannot save the upload"
		}
		_, err = io.Copy(f, part)
		f.Close()
		if errors.As(err, &mbe) {
			return nil, http.StatusRequestEntityTooLarge, tooBig
		}
		if err != nil {
			return nil, http.StatusBadRequest, "upload was interrupted"
		}
		if msg := a.checkFile(dst); msg != "" {
			return nil, http.StatusUnprocessableEntity, msg
		}
		job.files = append(job.files, dst)
	}
	if len(job.files) == 0 {
		return nil, http.StatusBadRequest, "no file received (use the \"file\" field)"
	}
	return job, 0, ""
}

// checkFile rejects files whose content doesn't match their extension and
// images so large that decoding them would use too much memory.
func (a *api) checkFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "cannot read the upload"
	}
	defer f.Close()

	if kindOf(path) == KindPDF {
		head := make([]byte, 1024)
		n, _ := io.ReadFull(f, head)
		if !strings.Contains(string(head[:n]), "%PDF-") {
			return filepath.Base(path) + " is not a valid PDF"
		}
		return ""
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return filepath.Base(path) + " is not a valid image"
	}
	if cfg.Width*cfg.Height > a.cfg.maxPixels {
		return fmt.Sprintf("%s is %dx%d; the limit is %d megapixels",
			filepath.Base(path), cfg.Width, cfg.Height, a.cfg.maxPixels/1_000_000)
	}
	return ""
}

// safeName keeps a readable file name but removes paths and odd characters.
func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, filepath.Ext(name))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		}
	}
	clean := strings.Trim(b.String(), "._")
	if len(clean) > 80 {
		clean = clean[:80]
	}
	if clean == "" {
		clean = "upload"
	}
	return clean + ext
}

func targetNames(ts []Target) string {
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

func zipFiles(out string, files []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, p := range files {
		w, err := zw.Create(filepath.Base(p))
		if err != nil {
			f.Close()
			return err
		}
		src, err := os.Open(p)
		if err != nil {
			f.Close()
			return err
		}
		_, err = io.Copy(w, src)
		src.Close()
		if err != nil {
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ---------- responses ----------

func sendFile(w http.ResponseWriter, path, downloadName string, originalSize int64) {
	f, err := os.Open(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "result file is missing")
		return
	}
	defer f.Close()
	st, _ := f.Stat()

	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h := w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadName}))
	h.Set("X-Fk-Original-Size", strconv.FormatInt(originalSize, 10))
	h.Set("X-Fk-Result-Size", strconv.FormatInt(st.Size(), 10))
	h.Set("Cache-Control", "no-store")
	io.Copy(w, f)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeOpErr(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoGhostscript) {
		writeErr(w, http.StatusServiceUnavailable, "PDF processing is not available on this server")
		return
	}
	writeErr(w, http.StatusUnprocessableEntity, err.Error())
}

// ---------- middleware ----------

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' blob: data:")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return // health checks run every few seconds; keep the log readable
		}
		log.Printf("%s %s %d %dB %s", r.Method, r.URL.Path, rec.status, rec.bytes, time.Since(start).Round(time.Millisecond))
	})
}
