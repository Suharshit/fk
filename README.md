# fk — File Kit

A small, fast tool written in Go for compressing and converting **PDF, PNG and JPG** files.

- **`fk`** — run it inside any folder. It lists the PDF/PNG/JPG files, you select with **Space**, press **Enter**, then pick **Compress**, **Convert** or **Merge into one PDF**.
- **`fk serve`** — opens a terminal in your browser (http://localhost:7700) that runs a real shell on your machine, so you can `cd` around your files and run `fk` from the web page.

It's a single binary of about 7 MB with no runtime dependencies. Ghostscript is only needed for PDF compression and for turning PDFs into images.

## Install (Ubuntu / WSL)

```bash
tar xzf fk.tar.gz
cd fk
mkdir -p ~/.local/bin
cp dist/fk ~/.local/bin/
sudo apt install -y ghostscript      # for PDF compress / PDF -> image
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

## Build from source

Needs Go 1.23+.

```bash
go build -o dist/fk .
go test ./...
```

## Project layout

| File | What it does |
|---|---|
| `main.go` | command-line entry point |
| `picker.go` | interactive file picker, menus, progress output |
| `ops.go` | compress / convert / merge logic |
| `imgutil.go` | EXIF rotation, colour quantization with dithering |
| `pdf.go` | image → PDF writer (pure Go), Ghostscript / Poppler calls |
| `server.go` | web server, WebSocket ↔ pseudo-terminal bridge |
| `web/` | browser page and bundled xterm.js (works offline) |
