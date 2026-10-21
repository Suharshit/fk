# fk — File Kit

[![ci](https://github.com/Suharshit/fk/actions/workflows/ci.yml/badge.svg)](https://github.com/Suharshit/fk/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/Suharshit/fk)](https://github.com/Suharshit/fk/releases)

A small, fast tool written in Go for compressing and converting **PDF, PNG and JPG** files.

- **`fk`** — run it inside any folder. It lists the PDF/PNG/JPG files, you select with **Space**, press **Enter**, then pick **Compress**, **Convert** or **Merge into one PDF**.
- **`fk serve`** — opens a terminal in your browser (http://localhost:7700) that runs a real shell on your machine, so you can `cd` around your files and run `fk` from the web page.
- **`fk api`** — a web app and HTTP API for servers: upload a file, get it back compressed or converted. Nothing is stored.

It comes with a full **DevOps pipeline**: GitHub Actions → Terraform → Ansible → Puppet → Nagios. See [docs/PIPELINE.md](docs/PIPELINE.md).

It's a single binary of about 7 MB with no runtime dependencies. Ghostscript is only needed for PDF compression and for turning PDFs into images.

## Install (Ubuntu / WSL)

```bash
mkdir -p ~/.local/bin
curl -fL https://github.com/Suharshit/fk/releases/latest/download/fk-linux-amd64 -o ~/.local/bin/fk
chmod +x ~/.local/bin/fk
sudo apt install -y ghostscript poppler-utils   # for PDF compress / PDF -> image
```

Open a new terminal (or run `source ~/.profile`) so `~/.local/bin` is on your `PATH`, then check:

```bash
fk version
```

## Use it

```bash
cd ~/Downloads        # any folder with PDFs or images
fk
```

| Key | Does |
|---|---|
| ↑ ↓ (or j k) | move |
| Space | select / unselect the file under the cursor |
| a | select all / none |
| Enter | continue (if nothing is selected, uses the file under the cursor) |
| Esc / q | back / quit |

Then choose:

| Action | What happens |
|---|---|
| **Compress** – Light | JPG quality 85 · PNG lossless re-pack · PDF 300 dpi images |
| **Compress** – Balanced | JPG quality 72, max 2560 px · PNG 256 colours (dithered) · PDF 150 dpi |
| **Compress** – Strong | JPG quality 55, max 1600 px · PNG 64 colours · PDF 72 dpi |
| **Convert** | images → PNG, JPG, PDF, GIF, BMP, TIFF · PDF → PNG or JPG (150 dpi, one image per page) |
| **Merge into one PDF** | 2+ images → `merged.pdf`, one image per page |

Your originals are never changed or overwritten. Results are saved next to them:

- `photo.jpg` → `photo_compressed.jpg` (then `photo_compressed-2.jpg`, …)
- `photo.jpg` → `photo.png`
- `report.pdf` (1 page) → `report.png`; (several pages) → `report_pages/page-01.png`, …

If compressing doesn't make a file smaller, the new copy is thrown away and the original is kept.

Phone photos that are stored sideways (EXIF rotation) are turned upright when re-saved.

### Without the picker

```bash
fk compress *.jpg --level strong
fk convert scan.pdf --to jpg
fk merge page1.jpg page2.jpg page3.png
```

## Web terminal

```bash
fk serve                 # starts in the current folder
fk serve --dir ~/Pictures --port 7800
```

It prints a link like `http://localhost:7700/?t=3f9c…`. Open it in Chrome or Edge on Windows (it tries to open the browser for you). Type `fk`, or press the **Run fk** button.

Safety:
- It only listens on `127.0.0.1`, so other computers on your network can't reach it.
- The link has a random token that changes every time, and connections from other websites are refused. Anyone with the link on your computer gets a shell as you, so don't share it.
- Ports 8080/8081 are left free for the Terraform practical; fk uses 7700.

## Web app and API (`fk api`)

```bash
fk api                      # http://localhost:8000
```

Open the page, drop files in, and choose **Compress**, **Convert** or **Merge**. Or use the API directly:

```bash
curl -F file=@report.pdf -F level=strong  http://localhost:8000/api/compress -o small.pdf
curl -F file=@photo.png  -F to=jpg        http://localhost:8000/api/convert  -o photo.jpg
curl -F file=@a.jpg -F file=@b.jpg        http://localhost:8000/api/merge    -o merged.pdf
curl http://localhost:8000/healthz        # ok
curl http://localhost:8000/version        # {"version":"v1.1.0", ...}
```

| Endpoint | Form fields | Returns |
|---|---|---|
| `POST /api/compress` | `file`, `level` = light / balanced / strong | the compressed file (the original if it can't be made smaller; see header `X-Fk-Note`) |
| `POST /api/convert` | `file`, `to` = png / jpg / pdf / gif / bmp / tiff | the converted file; a multi-page PDF comes back as a `.zip` of images |
| `POST /api/merge` | two or more `file` images | `merged.pdf` |
| `GET /healthz` | — | `ok` |
| `GET /version` | — | version, host name, whether Ghostscript is installed |

Settings (environment variables): `PORT` (8000), `FK_MAX_UPLOAD_MB` (25), `FK_MAX_JOBS` (CPU count), `FK_MAX_MEGAPIXELS` (60), `FK_MAX_PDF_PAGES` (50), `FK_WORKDIR` (system temp folder).

Every request works in its own temporary folder, which is deleted as soon as the response is sent. Uploads are checked for type, size and image dimensions, and Ghostscript runs in `-dSAFER` mode with a time limit.

## DevOps pipeline

| Stage | Tool | Where |
|---|---|---|
| CI on every push | GitHub Actions | `.github/workflows/ci.yml` |
| Release on every `v*` tag | GitHub Actions + Releases | `.github/workflows/release.yml` |
| Servers on AWS (or local VMs) | Terraform / Vagrant | `infra/terraform/`, `Vagrantfile` |
| Install and rolling deploys | Ansible | `infra/ansible/` |
| Keep servers in their declared state | Puppet (OpenVox) | `infra/puppet/` |
| Monitoring with a real upload check | Nagios | `infra/nagios/`, `infra/ansible/roles/nagios/` |

`make up` builds everything on AWS and `make down` removes it. The full walkthrough, demo script and syllabus mapping are in [docs/PIPELINE.md](docs/PIPELINE.md). The reusable template it was built from is [docs/DEVOPS_PIPELINE_TEMPLATE.md](docs/DEVOPS_PIPELINE_TEMPLATE.md).

## Build from source

Needs Go 1.23+.

```bash
make test
make build          # dist/fk-linux-amd64
make run-api        # build and start the web app on :8000
```

## Project layout

| File | What it does |
|---|---|
| `main.go` | command-line entry point |
| `picker.go` | interactive file picker, menus, progress output |
| `ops.go` | compress / convert / merge logic |
| `imgutil.go` | EXIF rotation, colour quantization with dithering |
| `pdf.go` | image → PDF writer (pure Go), Ghostscript / Poppler calls |
| `server.go` | `fk serve`: web terminal, WebSocket ↔ pseudo-terminal bridge |
| `api.go` | `fk api`: upload web app and HTTP API |
| `web/` | web pages (`app.html`, `index.html`) and bundled xterm.js |
| `infra/`, `.github/`, `Makefile`, `Vagrantfile` | the DevOps pipeline |
| `docs/` | pipeline guide and reusable template |
