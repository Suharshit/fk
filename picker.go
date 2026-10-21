package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// ---------- colours ----------

const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	inverse = "\x1b[7m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
)

func kindColor(k Kind) string {
	switch k {
	case KindPDF:
		return red
	case KindPNG:
		return blue
	case KindJPEG:
		return magenta
	}
	return ""
}

// ---------- files ----------

type fileItem struct {
	name string
	path string
	kind Kind
	size int64
}

func listFiles(dir string) ([]fileItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []fileItem
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		k := kindOf(e.Name())
		if k == KindOther {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, fileItem{e.Name(), filepath.Join(dir, e.Name()), k, info.Size()})
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].name) < strings.ToLower(files[j].name)
	})
	return files, nil
}

// ---------- terminal ----------

type key int

const (
	kNone key = iota
	kUp
	kDown
	kPgUp
	kPgDn
	kHome
	kEnd
	kSpace
	kEnter
	kEsc
	kQuit
	kChar
)

type screen struct {
	in    *os.File
	out   *bufio.Writer
	state *term.State
	ch    rune
}

func openScreen() (*screen, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, errors.New("fk needs an interactive terminal")
	}
	st, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	s := &screen{in: os.Stdin, out: bufio.NewWriter(os.Stdout), state: st}
	s.out.WriteString("\x1b[?1049h\x1b[?25l") // alternate screen, hide cursor
	s.out.Flush()
	return s, nil
}

func (s *screen) close() {
	if s.state == nil {
		return
	}
	s.out.WriteString("\x1b[?25h\x1b[?1049l") // show cursor, leave alternate screen
	s.out.Flush()
	term.Restore(int(s.in.Fd()), s.state)
	s.state = nil
}

func (s *screen) size() (w, h int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 20 || h < 8 {
		return 80, 24
	}
	return w, h
}

func (s *screen) readKey() key {
	buf := make([]byte, 32)
	n, err := s.in.Read(buf)
	if err != nil || n == 0 {
		return kQuit
	}
	b := buf[:n]
	switch {
	case n == 1 && b[0] == 0x1b:
		return kEsc
	case b[0] == 3 || b[0] == 4: // Ctrl+C, Ctrl+D
		return kQuit
	case b[0] == '\r' || b[0] == '\n':
		return kEnter
	case b[0] == ' ':
		return kSpace
	case n >= 3 && b[0] == 0x1b && (b[1] == '[' || b[1] == 'O'):
		switch string(b[2:]) {
		case "A":
			return kUp
		case "B":
			return kDown
		case "H", "1~", "7~":
			return kHome
		case "F", "4~", "8~":
			return kEnd
		case "5~":
			return kPgUp
		case "6~":
			return kPgDn
		}
		return kNone
	case b[0] == 0x1b:
		return kNone
	}
	r, _ := utf8.DecodeRune(b)
	s.ch = r
	switch r {
	case 'k':
		return kUp
	case 'j':
		return kDown
	}
	return kChar
}

// render clears the screen and prints lines, each cut to the terminal width.
func (s *screen) render(lines []string) {
	w, h := s.size()
	s.out.WriteString("\x1b[H\x1b[2J")
	for i, l := range lines {
		if i >= h {
			break
		}
		s.out.WriteString(truncateANSI(l, w))
		s.out.WriteString(reset)
		if i < len(lines)-1 {
			s.out.WriteString("\r\n")
		}
	}
	s.out.Flush()
}

// truncateANSI cuts a string with colour codes to max visible runes.
func truncateANSI(str string, max int) string {
	var b strings.Builder
	visible := 0
	for i := 0; i < len(str); {
		if str[i] == 0x1b {
			j := i + 1
			for j < len(str) && !((str[j] >= 'a' && str[j] <= 'z') || (str[j] >= 'A' && str[j] <= 'Z')) {
				j++
			}
			if j < len(str) {
				j++
			}
			b.WriteString(str[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(str[i:])
		if visible >= max {
			break
		}
		b.WriteRune(r)
		visible++
		i += size
	}
	return b.String()
}

func pad(s string, n int) string {
	if c := utf8.RuneCountInString(s); c < n {
		return s + strings.Repeat(" ", n-c)
	}
	return s
}

func shorten(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	if n < 4 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func header(dir string) []string {
	home, _ := os.UserHomeDir()
	shown := dir
	if home != "" && strings.HasPrefix(dir, home) {
		shown = "~" + strings.TrimPrefix(dir, home)
	}
	return []string{
		"",
		fmt.Sprintf("  %s fk %s  %s%s%s", bold+inverse+cyan, reset, dim, shown, reset),
		"",
	}
}

// ---------- step 1: choose files ----------

type picker struct {
	files    []fileItem
	selected map[int]bool
	cursor   int
	top      int
}

func (p *picker) run(s *screen, dir string) bool {
	for {
		_, h := s.size()
		rows := h - 8
		if rows < 3 {
			rows = 3
		}
		if p.cursor < p.top {
			p.top = p.cursor
		}
		if p.cursor >= p.top+rows {
			p.top = p.cursor - rows + 1
		}

		w, _ := s.size()
		nameW := w - 30
		if nameW < 12 {
			nameW = 12
		}

		lines := header(dir)
		lines = append(lines, bold+"  Choose files"+reset+dim+"  ("+fmt.Sprint(len(p.files))+" found)"+reset, "")
		end := p.top + rows
		if end > len(p.files) {
			end = len(p.files)
		}
		for i := p.top; i < end; i++ {
			f := p.files[i]
			pointer, mark := "   ", dim+"[ ]"+reset
			if p.selected[i] {
				mark = green + bold + "[x]" + reset
			}
			name := pad(shorten(f.name, nameW), nameW)
			if i == p.cursor {
				pointer = cyan + bold + " ❯ " + reset
				name = bold + name + reset
			}
			lines = append(lines, fmt.Sprintf(" %s%s %s%-3s%s  %s  %s%9s%s",
				pointer, mark, kindColor(f.kind), f.kind.Label(), reset, name, dim, humanSize(f.size), reset))
		}
		for i := end - p.top; i < rows; i++ {
			lines = append(lines, "")
		}
		more := ""
		if len(p.files) > rows {
			more = fmt.Sprintf("  %d-%d of %d", p.top+1, end, len(p.files))
		}
		lines = append(lines, dim+more+reset, "",
			fmt.Sprintf("  %s%d selected%s  %s↑↓ move · Space select · a all · Enter continue · q quit%s",
				green+bold, p.count(), reset, dim, reset))
		s.render(lines)

		switch s.readKey() {
		case kUp:
			if p.cursor > 0 {
				p.cursor--
			}
		case kDown:
			if p.cursor < len(p.files)-1 {
				p.cursor++
			}
		case kPgUp:
			p.cursor -= rows
			if p.cursor < 0 {
				p.cursor = 0
			}
		case kPgDn:
			p.cursor += rows
			if p.cursor > len(p.files)-1 {
				p.cursor = len(p.files) - 1
			}
		case kHome:
			p.cursor = 0
		case kEnd:
			p.cursor = len(p.files) - 1
		case kSpace:
			p.selected[p.cursor] = !p.selected[p.cursor]
		case kEnter:
			if p.count() == 0 {
				p.selected[p.cursor] = true
			}
			return true
		case kEsc, kQuit:
			return false
		case kChar:
			switch s.ch {
			case 'q', 'Q':
				return false
			case 'a', 'A':
				all := p.count() < len(p.files)
				for i := range p.files {
					p.selected[i] = all
				}
			}
		}
	}
}

func (p *picker) count() int {
	n := 0
	for _, v := range p.selected {
		if v {
			n++
		}
	}
	return n
}

func (p *picker) chosen() []fileItem {
	var out []fileItem
	for i, f := range p.files {
		if p.selected[i] {
			out = append(out, f)
		}
	}
	return out
}

// ---------- step 2+: menus ----------

type menuItem struct{ label, hint string }

// menu shows a list and returns the chosen index, or -1 for back.
func menu(s *screen, dir, title, sub string, items []menuItem, start int) int {
	cur := start
	width := 0
	for _, it := range items {
		if n := utf8.RuneCountInString(strings.TrimSpace(it.label)); n > width {
			width = n
		}
	}
	for {
		lines := header(dir)
		lines = append(lines, bold+"  "+title+reset)
		if sub != "" {
			lines = append(lines, dim+"  "+sub+reset)
		}
		lines = append(lines, "")
		for i, it := range items {
			label := pad(strings.TrimSpace(it.label), width)
			if i == cur {
				lines = append(lines, fmt.Sprintf("   %s❯ %s%s   %s%s%s", cyan+bold, label, reset, dim, it.hint, reset))
			} else {
				lines = append(lines, fmt.Sprintf("     %s   %s%s%s", label, dim, it.hint, reset))
			}
		}
		lines = append(lines, "", dim+"  ↑↓ move · Enter choose · Esc back"+reset)
		s.render(lines)

		switch s.readKey() {
		case kUp:
			if cur > 0 {
				cur--
			}
		case kDown:
			if cur < len(items)-1 {
				cur++
			}
		case kHome:
			cur = 0
		case kEnd:
			cur = len(items) - 1
		case kEnter, kSpace:
			return cur
		case kEsc:
			return -1
		case kQuit:
			return -2
		case kChar:
			if s.ch == 'q' {
				return -1
			}
			if s.ch >= '1' && s.ch <= '9' && int(s.ch-'1') < len(items) {
				return int(s.ch - '1')
			}
		}
	}
}

// ---------- the whole flow ----------

type job struct {
	kind   string // compress, convert, merge
	level  Level
	target Target
	files  []fileItem
}

func runPicker(dir string) error {
	dir, _ = filepath.Abs(dir)
	files, err := listFiles(dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Printf("No PDF, PNG or JPG files in %s\n", dir)
		return nil
	}

	s, err := openScreen()
	if err != nil {
		return err
	}
	// make sure the terminal is restored if we're killed
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sig
		s.close()
		os.Exit(1)
	}()

	j := chooseJob(s, dir, &picker{files: files, selected: map[int]bool{}})
	s.close()
	signal.Stop(sig)

	if j == nil {
		return nil
	}
	runJob(j)
	return nil
}

func chooseJob(s *screen, dir string, p *picker) *job {
	for {
		if !p.run(s, dir) {
			return nil
		}
		sel := p.chosen()
		var kinds []Kind
		images := 0
		for _, f := range sel {
			if !contains(kinds, f.kind) {
				kinds = append(kinds, f.kind)
			}
			if f.kind.isImage() {
				images++
			}
		}
		what := describe(sel)

		actions := []menuItem{
			{"Compress", "make the file smaller"},
			{"Convert", "save in another format"},
		}
		if images >= 2 && images == len(sel) {
			actions = append(actions, menuItem{"Merge into one PDF", fmt.Sprintf("%d images, one per page", images)})
		}
		actions = append(actions, menuItem{"Back", "choose other files"})

		act := 0
		for {
			act = menu(s, dir, "What should I do with "+what+"?", fileNames(sel), actions, act)
			if act == -2 {
				return nil
			}
			if act == -1 || actions[act].label == "Back" {
				break
			}
			switch actions[act].label {
			case "Compress":
				lv := menu(s, dir, "How much compression?", fileNames(sel), levelItems(kinds), 1)
				if lv == -2 {
					return nil
				}
				if lv >= 0 {
					return &job{kind: "compress", level: Level(lv), files: sel}
				}
			case "Convert":
				targets := commonTargets(kinds)
				items := make([]menuItem, len(targets))
				for i, t := range targets {
					parts := strings.SplitN(t.Label(), " ", 2)
					items[i] = menuItem{pad(parts[0], 5), strings.TrimSpace(parts[1])}
				}
				ti := menu(s, dir, "Convert "+what+" to…", fileNames(sel), items, 0)
				if ti == -2 {
					return nil
				}
				if ti >= 0 {
					return &job{kind: "convert", target: targets[ti], files: sel}
				}
			case "Merge into one PDF":
				return &job{kind: "merge", files: sel}
			}
		}
	}
}

func levelItems(kinds []Kind) []menuItem {
	hasPDF, hasImg := false, false
	for _, k := range kinds {
		if k == KindPDF {
			hasPDF = true
		} else {
			hasImg = true
		}
	}
	hint := func(img, pdf string) string {
		switch {
		case hasPDF && hasImg:
			return img + " · PDF: " + pdf
		case hasPDF:
			return pdf
		}
		return img
	}
	return []menuItem{
		{"Light    ", hint("best quality, JPG q85, PNG lossless", "print quality (300 dpi images)")},
		{"Balanced ", hint("recommended, JPG q72, PNG 256 colours, max 2560px", "screen & email (150 dpi)")},
		{"Strong   ", hint("smallest, JPG q55 max 1600px, PNG 64 colours", "smallest (72 dpi)")},
	}
}

func describe(sel []fileItem) string {
	if len(sel) == 1 {
		return sel[0].name
	}
	return fmt.Sprintf("%d files", len(sel))
}

func fileNames(sel []fileItem) string {
	names := make([]string, 0, len(sel))
	for _, f := range sel {
		names = append(names, f.name)
	}
	return strings.Join(names, ", ")
}

// ---------- doing the work ----------

// runDirect handles `fk compress|convert|merge FILE...`.
func runDirect(cmd string, args []string) int {
	j := &job{kind: cmd, level: Balanced}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--level", "-l", "--to", "-t":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "fk: %s needs a value\n", a)
				return 2
			}
			v := strings.ToLower(args[i+1])
			i++
			if a == "--level" || a == "-l" {
				switch v {
				case "light":
					j.level = Light
				case "balanced":
					j.level = Balanced
				case "strong":
					j.level = Strong
				default:
					fmt.Fprintf(os.Stderr, "fk: level must be light, balanced or strong\n")
					return 2
				}
			} else {
				if v == "jpeg" {
					v = "jpg"
				}
				if v == "tif" {
					v = "tiff"
				}
				j.target = Target(v)
			}
			continue
		}
		info, err := os.Stat(a)
		if err != nil || info.IsDir() {
			fmt.Fprintf(os.Stderr, "fk: %s: not a file\n", a)
			return 2
		}
		k := kindOf(a)
		if k == KindOther {
			fmt.Fprintf(os.Stderr, "fk: %s: only PDF, PNG and JPG files are supported\n", a)
			return 2
		}
		j.files = append(j.files, fileItem{filepath.Base(a), a, k, info.Size()})
	}
	if len(j.files) == 0 {
		fmt.Fprintf(os.Stderr, "fk: %s: no files given\n", cmd)
		return 2
	}
	switch cmd {
	case "convert":
		ok := false
		for _, t := range []Target{ToPNG, ToJPG, ToPDF, ToGIF, ToBMP, ToTIFF} {
			ok = ok || t == j.target
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "fk: convert needs --to png|jpg|pdf|gif|bmp|tiff")
			return 2
		}
		for _, f := range j.files {
			if !sameFormat(f.kind, j.target) && !contains(targetsFor(f.kind), j.target) {
				fmt.Fprintf(os.Stderr, "fk: %s cannot be converted to %s\n", f.name, j.target)
				return 2
			}
		}
	case "merge":
		for _, f := range j.files {
			if !f.kind.isImage() {
				fmt.Fprintf(os.Stderr, "fk: merge takes images only, %s is a PDF\n", f.name)
				return 2
			}
		}
	}
	if runJob(j) > 0 {
		return 1
	}
	return 0
}

func runJob(j *job) (failures int) {
	fmt.Println()
	var results []Result
	start := time.Now()

	switch j.kind {
	case "compress":
		fmt.Printf("  %sCompressing%s (%s)\n\n", bold, reset, j.level)
		for _, f := range j.files {
			progress(f.name)
			r := compressFile(f.path, j.level)
			report(r, true)
			results = append(results, r)
		}
	case "convert":
		fmt.Printf("  %sConverting to %s%s\n\n", bold, strings.ToUpper(string(j.target)), reset)
		for _, f := range j.files {
			progress(f.name)
			r := convertFile(f.path, j.target)
			report(r, false)
			results = append(results, r)
		}
	case "merge":
		fmt.Printf("  %sMerging %d images into one PDF%s\n\n", bold, len(j.files), reset)
		paths := make([]string, len(j.files))
		for i, f := range j.files {
			paths[i] = f.path
		}
		progress("merged.pdf")
		r := mergeToPDF(paths)
		report(r, false)
		results = append(results, r)
	}

	ok, failed := 0, 0
	var before, after int64
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
		case r.Out != "":
			ok++
			before += r.InSize
			after += r.OutSize
		}
	}
	fmt.Println()
	summary := fmt.Sprintf("  %s%d done%s", green+bold, ok, reset)
	if failed > 0 {
		summary += fmt.Sprintf(", %s%d failed%s", red+bold, failed, reset)
	}
	if j.kind == "compress" && before > 0 {
		summary += fmt.Sprintf("  ·  %s → %s saved %s", humanSize(before), humanSize(after), percent(before, after))
	}
	summary += fmt.Sprintf("  %s(%.1fs)%s", dim, time.Since(start).Seconds(), reset)
	fmt.Println(summary)
	fmt.Println()
	return failed
}

func progress(name string) {
	fmt.Printf("  %s…%s %s", dim, reset, name)
}

func report(r Result, showSaving bool) {
	fmt.Print("\r\x1b[K")
	in := filepath.Base(r.In)
	switch {
	case r.Err != nil:
		fmt.Printf("  %s✗%s %s  %s%v%s\n", red+bold, reset, in, red, r.Err, reset)
	case r.Discarded:
		fmt.Printf("  %s–%s %s  %s%s%s\n", yellow+bold, reset, in, dim, r.Note, reset)
	default:
		out := filepath.Base(r.Out)
		if len(r.Extra) > 0 {
			out = filepath.Base(filepath.Dir(r.Out)) + "/  (" + fmt.Sprint(len(r.Extra)+1) + " pages)"
		}
		fmt.Printf("  %s✓%s %s %s→%s %s%s%s  %s%s → %s%s",
			green+bold, reset, in, dim, reset, bold, out, reset,
			dim, humanSize(r.InSize), humanSize(r.OutSize), reset)
		if showSaving && r.OutSize < r.InSize {
			fmt.Printf("  %s%s%s", green, percent(r.InSize, r.OutSize), reset)
		}
		fmt.Println()
	}
}

func percent(before, after int64) string {
	if before == 0 {
		return ""
	}
	return fmt.Sprintf("-%d%%", int((before-after)*100/before))
}
