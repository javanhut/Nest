package gui

import (
	"fmt"
	"image/color"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/ssh"
)

const (
	termRows = 24
	termCols = 80
)

const (
	parseNormal = iota
	parseEsc
	parseCSI
	parseOSC
)

// Catppuccin Mocha palette
var (
	catBase     = color.RGBA{30, 30, 46, 255}
	catText     = color.RGBA{205, 214, 244, 255}
	catSubtext1 = color.RGBA{186, 194, 222, 255}
	catSubtext0 = color.RGBA{166, 173, 200, 255}
	catOverlay0 = color.RGBA{108, 112, 134, 255}
	catSurface2 = color.RGBA{88, 91, 112, 255}
	catSurface1 = color.RGBA{69, 71, 90, 255}
	catRed      = color.RGBA{243, 139, 168, 255}
	catGreen    = color.RGBA{166, 227, 161, 255}
	catYellow   = color.RGBA{249, 226, 175, 255}
	catBlue     = color.RGBA{137, 180, 250, 255}
	catPink     = color.RGBA{245, 194, 231, 255}
	catTeal     = color.RGBA{148, 226, 213, 255}
	catSky      = color.RGBA{137, 220, 235, 255}
	catSapphire = color.RGBA{116, 199, 236, 255}
	catMaroon   = color.RGBA{235, 160, 172, 255}
	catPeach    = color.RGBA{250, 179, 135, 255}
	catLavender = color.RGBA{180, 190, 254, 255}
	catRosewater = color.RGBA{245, 224, 220, 255}
)

// ANSI 16-color mapping to Catppuccin Mocha
var ansiColors = [16]color.Color{
	catSurface1, // 0  black
	catRed,      // 1  red
	catGreen,    // 2  green
	catYellow,   // 3  yellow
	catBlue,     // 4  blue
	catPink,     // 5  magenta
	catTeal,     // 6  cyan
	catSubtext1, // 7  white
	catSurface2, // 8  bright black
	catMaroon,   // 9  bright red
	catGreen,    // 10 bright green
	catYellow,   // 11 bright yellow
	catSapphire, // 12 bright blue
	catLavender, // 13 bright magenta
	catSky,      // 14 bright cyan
	catText,     // 15 bright white
}

type cell struct {
	ch rune
	fg color.Color
	bg color.Color
}

func defaultCell() cell {
	return cell{ch: ' ', fg: catText, bg: catBase}
}

type termWidget struct {
	widget.BaseWidget
	grid       *widget.TextGrid
	buf        [][]cell
	rows, cols int
	curR, curC int
	curFG      color.Color
	curBG      color.Color
	curBold    bool
	stdin      io.WriteCloser
	mu         sync.Mutex
	pState     int
	escBuf     []byte
}

func newTermWidget(rows, cols int, stdin io.WriteCloser) *termWidget {
	t := &termWidget{
		rows:  rows,
		cols:  cols,
		stdin: stdin,
		grid:  widget.NewTextGrid(),
		curFG: catText,
		curBG: catBase,
	}
	t.buf = make([][]cell, rows)
	for i := range t.buf {
		t.buf[i] = make([]cell, cols)
		for j := range t.buf[i] {
			t.buf[i][j] = defaultCell()
		}
	}
	t.ExtendBaseWidget(t)
	t.renderDirect()
	return t
}

func (t *termWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.grid)
}

func (t *termWidget) FocusGained() {}
func (t *termWidget) FocusLost()   {}

func (t *termWidget) TypedRune(r rune) {
	t.stdin.Write([]byte(string(r)))
}

func (t *termWidget) TypedKey(ev *fyne.KeyEvent) {
	var b []byte
	switch ev.Name {
	case fyne.KeyReturn:
		b = []byte("\r")
	case fyne.KeyBackspace:
		b = []byte{0x7f}
	case fyne.KeyTab:
		b = []byte("\t")
	case fyne.KeyEscape:
		b = []byte{0x1b}
	case fyne.KeyUp:
		b = []byte("\x1b[A")
	case fyne.KeyDown:
		b = []byte("\x1b[B")
	case fyne.KeyRight:
		b = []byte("\x1b[C")
	case fyne.KeyLeft:
		b = []byte("\x1b[D")
	case fyne.KeyHome:
		b = []byte("\x1b[H")
	case fyne.KeyEnd:
		b = []byte("\x1b[F")
	case fyne.KeyDelete:
		b = []byte("\x1b[3~")
	case fyne.KeyPageUp:
		b = []byte("\x1b[5~")
	case fyne.KeyPageDown:
		b = []byte("\x1b[6~")
	}
	if b != nil {
		t.stdin.Write(b)
	}
}

func (t *termWidget) newRow() []cell {
	row := make([]cell, t.cols)
	for j := range row {
		row[j] = defaultCell()
	}
	return row
}

func (t *termWidget) scrollUp() {
	copy(t.buf, t.buf[1:])
	t.buf[t.rows-1] = t.newRow()
}

func (t *termWidget) clearRegion(startR, startC, endR, endC int) {
	for i := startR; i <= endR && i < t.rows; i++ {
		jStart, jEnd := 0, t.cols
		if i == startR {
			jStart = startC
		}
		if i == endR {
			jEnd = endC
		}
		for j := jStart; j < jEnd && j < t.cols; j++ {
			t.buf[i][j] = defaultCell()
		}
	}
}

func (t *termWidget) handleSGR(args []int) {
	if len(args) == 0 {
		args = []int{0}
	}
	for i := 0; i < len(args); i++ {
		p := args[i]
		switch {
		case p == 0:
			t.curFG = catText
			t.curBG = catBase
			t.curBold = false
		case p == 1:
			t.curBold = true
		case p == 2 || p == 22:
			t.curBold = false
		case p == 7: // reverse video
			t.curFG, t.curBG = t.curBG, t.curFG
		case p == 27: // reverse off
			t.curFG = catText
			t.curBG = catBase
		case p >= 30 && p <= 37:
			idx := p - 30
			if t.curBold {
				idx += 8
			}
			t.curFG = ansiColors[idx]
		case p == 38: // extended foreground
			if i+1 < len(args) && args[i+1] == 5 && i+2 < len(args) {
				t.curFG = color256(args[i+2])
				i += 2
			} else if i+1 < len(args) && args[i+1] == 2 && i+4 < len(args) {
				t.curFG = color.RGBA{uint8(args[i+2]), uint8(args[i+3]), uint8(args[i+4]), 255}
				i += 4
			}
		case p == 39:
			t.curFG = catText
		case p >= 40 && p <= 47:
			t.curBG = ansiColors[p-40]
		case p == 48: // extended background
			if i+1 < len(args) && args[i+1] == 5 && i+2 < len(args) {
				t.curBG = color256(args[i+2])
				i += 2
			} else if i+1 < len(args) && args[i+1] == 2 && i+4 < len(args) {
				t.curBG = color.RGBA{uint8(args[i+2]), uint8(args[i+3]), uint8(args[i+4]), 255}
				i += 4
			}
		case p == 49:
			t.curBG = catBase
		case p >= 90 && p <= 97:
			t.curFG = ansiColors[p-90+8]
		case p >= 100 && p <= 107:
			t.curBG = ansiColors[p-100+8]
		}
	}
}

func (t *termWidget) handleCSI(params string, final byte) {
	args := parseCSIArgs(params)
	switch final {
	case 'A':
		n := argDefault(args, 0, 1)
		t.curR = max(0, t.curR-n)
	case 'B':
		n := argDefault(args, 0, 1)
		t.curR = min(t.rows-1, t.curR+n)
	case 'C':
		n := argDefault(args, 0, 1)
		t.curC = min(t.cols-1, t.curC+n)
	case 'D':
		n := argDefault(args, 0, 1)
		t.curC = max(0, t.curC-n)
	case 'H', 'f':
		r := clamp(argDefault(args, 0, 1)-1, 0, t.rows-1)
		c := clamp(argDefault(args, 1, 1)-1, 0, t.cols-1)
		t.curR, t.curC = r, c
	case 'J':
		switch argDefault(args, 0, 0) {
		case 0:
			t.clearRegion(t.curR, t.curC, t.rows-1, t.cols)
		case 1:
			t.clearRegion(0, 0, t.curR, t.curC+1)
		case 2, 3:
			t.clearRegion(0, 0, t.rows-1, t.cols)
		}
	case 'K':
		switch argDefault(args, 0, 0) {
		case 0:
			t.clearRegion(t.curR, t.curC, t.curR, t.cols)
		case 1:
			t.clearRegion(t.curR, 0, t.curR, t.curC+1)
		case 2:
			t.clearRegion(t.curR, 0, t.curR, t.cols)
		}
	case 'G':
		t.curC = clamp(argDefault(args, 0, 1)-1, 0, t.cols-1)
	case 'd':
		t.curR = clamp(argDefault(args, 0, 1)-1, 0, t.rows-1)
	case 'm':
		t.handleSGR(args)
	case 'h', 'l', 'r':
		// mode set/reset, scroll region — ignored
	}
}

func (t *termWidget) processOutput(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, b := range data {
		switch t.pState {
		case parseNormal:
			switch b {
			case 0x1b:
				t.pState = parseEsc
				t.escBuf = nil
			case '\r':
				t.curC = 0
			case '\n':
				t.curR++
				if t.curR >= t.rows {
					t.scrollUp()
					t.curR = t.rows - 1
				}
			case '\b':
				if t.curC > 0 {
					t.curC--
				}
			case '\t':
				t.curC = min((t.curC/8+1)*8, t.cols-1)
			case '\a':
			default:
				if b >= 32 {
					if t.curC >= t.cols {
						t.curC = 0
						t.curR++
						if t.curR >= t.rows {
							t.scrollUp()
							t.curR = t.rows - 1
						}
					}
					t.buf[t.curR][t.curC] = cell{ch: rune(b), fg: t.curFG, bg: t.curBG}
					t.curC++
				}
			}
		case parseEsc:
			switch b {
			case '[':
				t.pState = parseCSI
				t.escBuf = nil
			case ']':
				t.pState = parseOSC
			default:
				t.pState = parseNormal
			}
		case parseCSI:
			if b >= 0x40 && b <= 0x7e {
				t.handleCSI(string(t.escBuf), b)
				t.pState = parseNormal
			} else {
				t.escBuf = append(t.escBuf, b)
			}
		case parseOSC:
			if b == '\a' || b == 0x1b {
				t.pState = parseNormal
			}
		}
	}

	rows := t.buildRows()
	fyne.Do(func() {
		t.grid.Rows = rows
		t.grid.Refresh()
	})
}

func (t *termWidget) buildRows() []widget.TextGridRow {
	rows := make([]widget.TextGridRow, t.rows)
	for i := 0; i < t.rows; i++ {
		cells := make([]widget.TextGridCell, t.cols)
		for j := 0; j < t.cols; j++ {
			c := t.buf[i][j]
			fg, bg := c.fg, c.bg
			if i == t.curR && j == t.curC {
				fg, bg = bg, fg // invert for cursor
			}
			cells[j] = widget.TextGridCell{
				Rune:  c.ch,
				Style: &widget.CustomTextGridStyle{FGColor: fg, BGColor: bg},
			}
		}
		rows[i] = widget.TextGridRow{Cells: cells}
	}
	return rows
}

func (t *termWidget) renderDirect() {
	t.grid.Rows = t.buildRows()
	t.grid.Refresh()
}

func (t *termWidget) readLoop(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			t.processOutput(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func color256(idx int) color.Color {
	if idx < 16 {
		return ansiColors[idx]
	}
	if idx < 232 {
		idx -= 16
		r := uint8((idx / 36) * 51)
		g := uint8(((idx / 6) % 6) * 51)
		b := uint8((idx % 6) * 51)
		return color.RGBA{r, g, b, 255}
	}
	v := uint8(8 + (idx-232)*10)
	return color.RGBA{v, v, v, 255}
}

func parseCSIArgs(s string) []int {
	s = strings.TrimPrefix(s, "?")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	args := make([]int, len(parts))
	for i, p := range parts {
		args[i], _ = strconv.Atoi(p)
	}
	return args
}

func argDefault(args []int, idx, def int) int {
	if idx < len(args) && args[idx] != 0 {
		return args[idx]
	}
	return def
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// OpenTerminal opens a Fyne window with an interactive SSH terminal session.
// cleanup is called when the window closes to tear down the full connection chain.
func OpenTerminal(client *ssh.Client, cleanup func()) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", termRows, termCols, modes); err != nil {
		session.Close()
		return fmt.Errorf("failed to request pty: %w", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	a := app.New()
	w := a.NewWindow("Nest - SSH Terminal")
	w.Resize(fyne.NewSize(800, 600))

	term := newTermWidget(termRows, termCols, stdinPipe)

	bg := canvas.NewRectangle(catBase)
	content := container.NewStack(bg, term)
	w.SetContent(content)
	w.Canvas().Focus(term)

	go term.readLoop(stdoutPipe)

	go func() {
		if err := session.Wait(); err != nil {
			log.Printf("Session ended: %v", err)
		}
		fyne.Do(func() {
			a.Quit()
		})
	}()

	w.SetOnClosed(func() {
		session.Close()
		cleanup()
	})

	w.ShowAndRun()
	return nil
}
