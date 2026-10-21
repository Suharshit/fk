// fk is a small file toolkit for the terminal.
//
//	fk          pick files in the current folder, then compress or convert them
//	fk serve    open a web terminal in your browser, connected to this machine
//	fk api      run the upload web app + HTTP API (for servers)
package main

import (
	"fmt"
	"os"
)

// version is set at build time: go build -ldflags "-X main.version=v1.2.0"
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		dir, err := os.Getwd()
		if err != nil {
			fail(err)
		}
		if err := runPicker(dir); err != nil {
			fail(err)
		}
		return
	}

	switch args[0] {
	case "serve", "web":
		if err := runServe(args[1:]); err != nil {
			fail(err)
		}
	case "api":
		if err := runAPI(args[1:]); err != nil {
			fail(err)
		}
	case "compress", "convert", "merge":
		os.Exit(runDirect(args[0], args[1:]))
	case "-h", "--help", "help":
		printHelp()
	case "-v", "--version", "version":
		fmt.Println("fk", version)
	default:
		// "fk some/folder" opens the picker in that folder.
		if st, err := os.Stat(args[0]); err == nil && st.IsDir() {
			if err := runPicker(args[0]); err != nil {
				fail(err)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "fk: unknown command %q\n\n", args[0])
		printHelp()
		os.Exit(2)
	}
}

func printHelp() {
	fmt.Print(`fk - compress and convert PDF, PNG and JPG files

Usage:
  fk                 pick files in the current folder and choose what to do
  fk <folder>        same, but in another folder
  fk serve           start the web terminal (http://localhost:7700)
      --port N       use another port (default 7700)
      --dir PATH     folder the web terminal starts in (default: current)
      --no-open      don't try to open the browser
  fk api             run the upload web app + HTTP API (port 8000)
      --port N       port (default 8000, or $PORT)
      --addr IP      address to listen on (default 0.0.0.0)
  fk version         print the version

Without the picker (for scripts):
  fk compress FILE... [--level light|balanced|strong]
  fk convert  FILE... --to png|jpg|pdf|gif|bmp|tiff
  fk merge    IMAGE...            put images into one PDF

In the picker:
  Up/Down or j/k     move
  Space              select / unselect a file
  a                  select all / none
  Enter              continue
  Esc or q           go back / quit

Compressing PDFs and converting PDFs to images needs Ghostscript:
  sudo apt install ghostscript
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fk:", err)
	os.Exit(1)
}
