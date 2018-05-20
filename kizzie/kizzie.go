package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	k ".."
	sdl "github.com/veandco/go-sdl2/sdl"
)

const TITLE = "Kikashi"

// -------------------------------------------------------------------------

type Variation struct {
	Rank		int
	Score		float64
	List		[]k.Move
}

func (self Variation) String() string {

	if len(self.List) == 0 {
		return ""
	}

	var lines []string
	var i int

	for {

		if len(lines) == 0 {
			lines = append(lines, "")
		} else {
			lines = append(lines, "        ")
		}

		for len(lines[len(lines) - 1]) < 70 {

			mv := self.List[i]

			lines[len(lines) - 1] += fmt.Sprintf("%v ", &mv)

			i++
			if i >= len(self.List) {
				return strings.Join(lines, "\n")
			}
		}
	}
}

// -------------------------------------------------------------------------

type LineBuffer struct {
	sync.Mutex
	Reader			io.Reader
	Lines			[]string
}

func (self *LineBuffer) Loop() {
	scanner := bufio.NewScanner(self.Reader)
	for scanner.Scan() {
		self.Lock()
		self.Lines = append(self.Lines, scanner.Text())
		self.Unlock()
	}
}

func (self *LineBuffer) Dump() []string {

	var new_lines []string

	self.Lock()
	for _, line := range self.Lines {
		new_lines = append(new_lines, line)
	}
	self.Lines = nil
	self.Unlock()

	return new_lines
}

// -------------------------------------------------------------------------

type App struct {
	Window				*sdl.Window
	Renderer			*sdl.Renderer
	PixelWidth			int
	PixelHeight			int
	CellWidth			int
	Margin				int
	Offset				int

	Node				*k.Node
	EngineNode			*k.Node

	LZ_Cmd				*exec.Cmd

	LZ_Stdin			io.Writer
	LZ_Stdout_Buffer	LineBuffer
	LZ_Stderr_Buffer	LineBuffer

	Variations			[]*Variation
	VariationsNext		[]*Variation

	NextAccept			time.Time

	MouseX				int
	MouseY				int
}


func NewApp(SZ, cell_width, margin int) *App {

	self := new(App)
	self.Node = k.NewTree(19)

	self.CellWidth = cell_width
	self.Offset = self.CellWidth / 2
	self.Margin = margin

	self.PixelWidth = (self.CellWidth * self.Node.Size()) + (self.Margin * 2)
	self.PixelHeight = self.PixelWidth

	self.InitSDL()

	self.LZ_Cmd = exec.Command("./leelaz.exe", "--gtp", "-w", "network")

	self.LZ_Stdin, _ = self.LZ_Cmd.StdinPipe()
	self.LZ_Stdout_Buffer.Reader, _ = self.LZ_Cmd.StdoutPipe()
	self.LZ_Stderr_Buffer.Reader, _ = self.LZ_Cmd.StderrPipe()

	err := self.LZ_Cmd.Start()

	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}

	go self.LZ_Stdout_Buffer.Loop()
	go self.LZ_Stderr_Buffer.Loop()

	return self
}


func (self *App) InitSDL() {

	fmt.Printf("Init...\n")

	err := sdl.Init(sdl.INIT_EVERYTHING)
	if err != nil {
		panic(err)
	}

	fmt.Printf("CreateWindow...\n")

	self.Window, err = sdl.CreateWindow(
		TITLE, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, int32(self.PixelWidth), int32(self.PixelHeight), sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}

	fmt.Printf("CreateRenderer...\n")

	self.Renderer, err = sdl.CreateRenderer(self.Window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		panic(err)
	}
}


func (self *App) Shutdown() {
	fmt.Printf("Kill...\n")
	self.LZ_Cmd.Process.Kill()
	fmt.Printf("Destroy...\n")
	self.Renderer.Destroy()
	fmt.Printf("Destroy...\n")
	self.Window.Destroy()
	fmt.Printf("Quit...\n")
	sdl.Quit()
}


func (self *App) Flip() {
	self.Renderer.Present()
}


func (self *App) Cls(r, g, b uint8) {
	self.Renderer.SetDrawColor(r, g, b, 255)
	self.Renderer.FillRect(&sdl.Rect{0, 0, int32(self.PixelWidth), int32(self.PixelHeight)})
}


func (self *App) PixelXY(x, y int) (int, int) {
	retx := x * self.CellWidth + self.Offset + self.Margin
	rety := y * self.CellWidth + self.Offset + self.Margin
	return retx, rety
}


func (self *App) BoardXY(x1, y1 int, clamp bool) (int, int) {

	min := self.Offset + self.Margin - (self.CellWidth / 2)
	max := ((self.Node.Size() - 1) * self.CellWidth) + self.Offset + self.Margin + (self.CellWidth / 2)

	diff := float64(max - min)

	retx_f := (float64(x1 - min) / diff) * float64(self.Node.Size())
	rety_f := (float64(y1 - min) / diff) * float64(self.Node.Size())

	retx := int(math.Floor(retx_f))
	rety := int(math.Floor(rety_f))

	if clamp {
		if retx < 0 { retx = 0 }
		if retx >= self.Node.Size() { retx = self.Node.Size() - 1 }
		if rety < 0 { rety = 0 }
		if rety >= self.Node.Size() { rety = self.Node.Size() - 1 }
	}

	return retx, rety
}


func (self *App) AllHoshi() []k.Point {
	// FIXME - generalise
	return []k.Point{{3, 3}, {3, 9}, {3, 15}, {9, 3}, {9, 9}, {9, 15}, {15, 3}, {15, 9}, {15, 15}}
}


func (self *App) DrawGrid() {

	self.Cls(208, 172, 114)

	self.Renderer.SetDrawColor(0, 0, 0, 255)

	for x := 0 ; x < self.Node.Size() ; x++ {
		x1, y1 := self.PixelXY(x, 0)
		x2, y2 := self.PixelXY(x, self.Node.Size() - 1)
		self.Renderer.DrawLine(int32(x1), int32(y1), int32(x2), int32(y2))
	}

	for y := 0 ; y < self.Node.Size() ; y++ {
		x1, y1 := self.PixelXY(0, y)
		x2, y2 := self.PixelXY(self.Node.Size() - 1, y)
		self.Renderer.DrawLine(int32(x1), int32(y1), int32(x2), int32(y2))
	}

	for _, hoshi := range self.AllHoshi() {
		x, y := self.PixelXY(hoshi.X, hoshi.Y)
		self.Renderer.DrawRect(&sdl.Rect{int32(x - 1), int32(y - 1), int32(3), int32(3)})
	}
}


func (self *App) DrawBoard(v *Variation, show_starts bool) {

	self.DrawGrid()

	// Draw known stones in the board (includes stones from B, W, AB, AW

	for x := 0; x < self.Node.Size(); x++ {

		for y := 0; y < self.Node.Size(); y++ {

			if self.Node.Board[x][y] != k.EMPTY {

				x1, y1 := self.PixelXY(x, y)

				if self.Node.Board[x][y] == k.BLACK {
					self.Fcircle(x1, y1, self.CellWidth / 2, 0, 0, 0)
				} else if self.Node.Board[x][y] == k.WHITE {
					self.Fcircle(x1, y1, self.CellWidth / 2, 255, 255, 255)
					self.Circle(x1, y1, self.CellWidth / 2, 0, 0, 0)
				}
			}
		}
	}

	// Draw a marker on this node's move...

	move_info := self.Node.MoveInfo()

	if move_info.OK && move_info.Pass == false {
		x1, y1 := self.PixelXY(move_info.X, move_info.Y)
		if move_info.Colour == k.WHITE {
			self.Fcircle(x1, y1, self.CellWidth / 8, 0, 0, 0)
		} else {
			self.Fcircle(x1, y1, self.CellWidth / 8, 255, 255, 255)
		}
	}

	// Draw the variation we've been give, if any...

	if v != nil {

		for _, mv := range v.List {

			if mv.OK && mv.Pass == false {

				x1, y1 := self.PixelXY(mv.X, mv.Y)

				if self.Node.Board[mv.X][mv.Y] == k.EMPTY {
					if mv.Colour == k.BLACK {
						self.Fcircle(x1, y1, self.CellWidth / 2, 0, 0, 0)
					} else {
						self.Fcircle(x1, y1, self.CellWidth / 2, 255, 255, 255)
						self.Circle(x1, y1, self.CellWidth / 2, 0, 0, 0)
					}
				}
			}
		}
	}

	if show_starts {
		for i, variation := range self.Variations {
			if len(variation.List) > 0 {
				if variation.List[0].OK && variation.List[0].Pass == false {
					x1, y1 := self.PixelXY(variation.List[0].X, variation.List[0].Y)

					if i == 0 {
						self.Circle(x1, y1, self.CellWidth / 2, 0, 128, 255)
						self.Circle(x1, y1, self.CellWidth / 2 - 1, 0, 128, 255)
					} else {
						self.Circle(x1, y1, self.CellWidth / 2, 255, 0, 0)
						self.Circle(x1, y1, self.CellWidth / 2 - 1, 255, 0, 0)
					}
				}
			}
		}
	}

	self.Flip()
}


func (self *App) Circle(x, y, radius int, r, g, b uint8) {

	self.Renderer.SetDrawColor(r, g, b, 255)

	var pyth float64
	var topline bool = true
	var lastiplusone int

	for j := radius - 1; j >= 0; j-- {
		for i := radius - 1; i >= 0; i-- {
			pyth = math.Sqrt(math.Pow(float64(i), 2) + math.Pow(float64(j), 2))
			if (pyth < float64(radius) - 0.5) {
				if topline {                    // i.e. if we're on the top (and, with mirroring, bottom) lines
					topline = false
					self.Renderer.DrawLine(int32(x - i - 1), int32(y - j - 1), int32(x + i), int32(y - j - 1))
					self.Renderer.DrawLine(int32(x - i - 1), int32(y + j), int32(x + i), int32(y + j))
					lastiplusone = i + 1
				} else {
					if lastiplusone == i + 1 {
						self.Renderer.DrawPoint(int32(x - i - 1), int32(y - j - 1))
						self.Renderer.DrawPoint(int32(x + i), int32(y - j - 1))
						self.Renderer.DrawPoint(int32(x - i - 1), int32(y + j))
						self.Renderer.DrawPoint(int32(x + i), int32(y + j))
					} else {
						self.Renderer.DrawLine(int32(x - i - 1), int32(y - j - 1), int32(x - lastiplusone - 1), int32(y - j - 1))
						self.Renderer.DrawLine(int32(x + lastiplusone), int32(y - j - 1), int32(x + i), int32(y - j - 1))
						self.Renderer.DrawLine(int32(x - i - 1), int32(y + j), int32(x - lastiplusone - 1), int32(y + j))
						self.Renderer.DrawLine(int32(x + lastiplusone), int32(y + j), int32(x + i), int32(y + j))
						lastiplusone = i + 1
					}
				}
				break
			}
		}
	}
}


func (self *App) Fcircle(x, y, radius int, r, g, b uint8) {

	var pyth float64;

	self.Renderer.SetDrawColor(r, g, b, 255)

	for j := radius; j >= 0; j-- {
		for i:= radius; i >= 0; i-- {
			pyth = math.Sqrt(math.Pow(float64(i), 2) + math.Pow(float64(j), 2));
			if (pyth < float64(radius) - 0.5) {
				self.Renderer.DrawLine(int32(x - i - 1), int32(y - j - 1), int32(x + i), int32(y - j - 1))
				self.Renderer.DrawLine(int32(x - i - 1), int32(y + j), int32(x + i), int32(y + j))
				break
			}
		}
	}
}


func (self *App) PollNoAction() {
	for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
		// Take no action. SDL still cleans itself up I think.
	}
}


func (self *App) Poll() {

	for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {

		switch event := ev.(type) {

		case *sdl.QuitEvent:

			self.Shutdown()
			os.Exit(0)

		case *sdl.MouseButtonEvent:

			if event.Type == sdl.MOUSEBUTTONDOWN {

				x, y := self.BoardXY(int(event.X), int(event.Y), true)

				new_node, err := self.Node.TryMove(self.Node.NextColour(), x, y)
				if err != nil {
					// fmt.Printf("%v\n", err)
				} else {
					self.Node = new_node
					self.Sync()
				}
			}

		case *sdl.MouseWheelEvent:

			if event.Y > 0 {

				if self.Node.Parent != nil {
					self.Node = self.Node.Parent
					self.Sync()
				}
			}

			if event.Y < 0 {		// Ideally we'd remember which line of the game we've been in.

				if len(self.Node.Children) > 0 {
					self.Node = self.Node.Children[0]
					self.Sync()
				}
			}

		case *sdl.MouseMotionEvent:

			self.MouseX, self.MouseY = self.BoardXY(int(event.X), int(event.Y), false)		// OK to be outside 0-18

		case *sdl.KeyboardEvent:

			if event.Type == sdl.KEYDOWN {

				switch event.Keysym.Sym {

				// https://github.com/veandco/go-sdl2/blob/master/sdl/keycode.go

				case sdl.K_s:

					var filename string
					var dialog_done bool

					result_chan := make(chan string)
					go file_dialog(true, result_chan)

					// Keep consuming events and stderr...

					for !dialog_done {

						select {

						case filename = <- result_chan:
							dialog_done = true

						default:
							self.PollNoAction()
							self.Analyse()			// Keep consuming stderr
						}
					}

					if filename == "" {
						break
					}

					self.Node.Save(filename)

				case sdl.K_o:

					var filename string
					var dialog_done bool

					result_chan := make(chan string)
					go file_dialog(false, result_chan)

					// Keep consuming events and stderr...

					for !dialog_done {

						select {

						case filename = <- result_chan:
							dialog_done = true

						default:
							self.PollNoAction()
							self.Analyse()			// Keep consuming stderr
						}
					}

					if filename == "" {
						break
					}

					new_root, err := k.Load(string(filename))
					if err != nil {
						fmt.Printf("%v\n", err)
						break
					}

					self.Node = new_root
					self.Sync()

				case sdl.K_p:

					self.Node = self.Node.TryPass(self.Node.NextColour())
					self.Sync()

				case sdl.K_RETURN:

					if len(self.Variations) > 0 {

						if len(self.Variations[0].List) > 0 {

							mv := self.Variations[0].List[0]

							if mv.OK && mv.Pass == false {

								new_node, err := self.Node.TryMove(self.Node.NextColour(), mv.X, mv.Y)
								if err != nil {
									// fmt.Printf("%v\n", err)
								} else {
									self.Node = new_node
									self.Sync()
								}
							}
						}
					}

				case sdl.K_m:

					fmt.Printf("--------------------\n")
					for _, v := range self.Variations {
						fmt.Printf("%v\n", v)
					}

				case sdl.K_g:

					for _, c := range self.Node.FullGTP() {
						fmt.Printf("%s\n", c)
					}

				case sdl.K_END:

					self.Node = self.Node.GetEnd()
					self.Sync()

				case sdl.K_HOME:

					self.Node = self.Node.GetRoot()
					self.Sync()

				case sdl.K_DOWN:

					if len(self.Node.Children) > 0 {
						self.Node = self.Node.Children[0]
						self.Sync()
					}

				case sdl.K_UP:

					if self.Node.Parent != nil {
						self.Node = self.Node.Parent
						self.Sync()
					}
				}
			}
		}
	}
}


func (self *App) Analyse() {

	// Just updates the self.Variations and self.VariationsNext maps.

	new_lines := self.LZ_Stderr_Buffer.Dump()

	// Ignore for a little while after Syncing so we don't get old info...

	if time.Now().Before(self.NextAccept) {
		return
	}

	for _, line := range new_lines {

		if line == "~end" {
			self.Variations = self.VariationsNext
			self.VariationsNext = nil
		} else {
			pv := pv_from_line(line, self.Node.NextColour(), self.Node.Size())

			if len(pv.List) > 0 {
				self.VariationsNext = append(self.VariationsNext, pv)
				pv.Rank = len(self.VariationsNext)						// 1 being the best
			}
		}

		// fmt.Printf("%s\n", line)
	}
}


func (self *App) Run() {

	self.SendToEngine("time_left B 0 0")

	self.DrawBoard(nil, true)

	for {
		self.Poll()
		self.Analyse()

		var v *Variation

		for _, variation := range self.Variations {
			if len(variation.List) > 0 && variation.List[0].X == self.MouseX && variation.List[0].Y == self.MouseY {
				v = variation
			}
		}

		if v != nil {
			self.DrawBoard(v, false)
		} else {
			self.DrawBoard(nil, true)
		}

		self.LZ_Stdout_Buffer.Dump()

		// FIXME: sleep?
	}
}


func (self *App) SendToEngine(cmd string) {
	cmd = strings.TrimSpace(cmd)
	self.LZ_Stdin.Write([]byte(cmd))
	self.LZ_Stdin.Write([]byte{'\n'})
	fmt.Printf("%s\n", string(cmd))
}


func (self *App) Sync() {

	if self.Node == self.EngineNode {
		return
	}

	if self.EngineNode != nil && self.EngineNode.Parent == self.Node {

		// We went up...

		for _, _ = range self.EngineNode.StepGTP() {
			self.SendToEngine("undo")
		}

	} else if self.Node != nil && self.Node.Parent == self.EngineNode {

		// We went down...

		for _, cmd := range self.Node.StepGTP() {
			self.SendToEngine(cmd)
		}

	} else {

		// We got lost...

		for _, cmd := range self.Node.FullGTP() {
			self.SendToEngine(cmd)
		}
	}

	self.EngineNode = self.Node
	self.SendToEngine("time_left B 0 0")

	self.Variations = nil
	self.VariationsNext = nil
	self.NextAccept = time.Now().Add(100 * time.Millisecond)
}


// -------------------------------------------------------------------------

func main() {
	app := NewApp(19, 36, 20)
	app.Run()
}


func file_dialog(save bool, result_chan chan string) {

	exe_dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	script_path := filepath.Join(exe_dir, "filedialog.py")

	all_python_args := []string{script_path}

	if save {
		all_python_args = append(all_python_args, "save")
	}

	subprocess_output, err := exec.Command("python", all_python_args...).Output()

	if err != nil {
		fmt.Printf("file_dialog(): %v\n", err)
		result_chan <- ""
		return
	}

	s := string(subprocess_output)
	s = strings.Replace(s, "\r", "", -1)
	s = strings.Replace(s, "\n", "", -1)

	if s == "" {
		result_chan <- ""
		return
	}

	result_chan <- s
}


func pv_from_line(s string, next_colour k.Colour, size int) *Variation {

	tokens := strings.Fields(s)

	v := new(Variation)

	if len(tokens) < 9 || tokens[7] != "PV:" {
		return v		// Zeroed
	}

	for _, t := range tokens[8:] {

		var mv k.Move

		if t == "pass" {

			mv = k.Move{
				OK: true,
				Pass: true,
				Colour: next_colour,
				Size: size,
			}

		} else {

			x, y, ok := k.PointFromHumanString(t, size)

			if ok {
				mv = k.Move{
					OK: true,
					Colour: next_colour,
					X: x,
					Y: y,
					Size: size,
				}
			} else {
				fmt.Printf("Warning: k.PointFromHumanString() returned ok: false")
			}
		}

		if mv.OK {
			v.List = append(v.List, mv)
			next_colour = next_colour.Opposite()
		}
	}

	return v
}
