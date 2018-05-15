package main

import (
	"fmt"
	"os"

	"github.com/veandco/go-sdl2/sdl"
)

const TITLE = "Kikashi"

// -------------------------------------------------------------------------

type Colour int

const (
	EMPTY = Colour(iota)
	BLACK
	WHITE
)

type Point struct {
	X				int32
	Y				int32
}

type Move struct {
	OK				bool
	Pass			bool
	Colour			Colour
	X				int
	Y				int
}

// -------------------------------------------------------------------------
// Nodes in an SGF tree...

type Node struct {
	Props			map[string][]string
	Children		[]*Node
	Parent			*Node
}


func NewNode(parent *Node) *Node {
	node := new(Node)
	node.Props = make(map[string][]string)
	node.Parent = parent

	if parent != nil {
		parent.Children = append(parent.Children, node)
	}

	return node
}


func (self *Node) AddValue(key, value string) {

	for i := 0; i < len(self.Props[key]); i++ {
		if self.Props[key][i] == value {
			return
		}
	}

	self.Props[key] = append(self.Props[key], value)
}


func (self *Node) GetValue(key string) (value string, ok bool) {

	// Get the value for the key, on the assumption that there's only 1 value.

	list := self.Props[key]

	if len(list) == 0 {
		return "", false
	}

	return list[0], true
}


func (self *Node) MoveInfo(size int) Move {

	// There should only be 1 move in a valid SGF node.

	for _, foo := range self.Props["B"] {

		x, y, valid := point_from_string(foo, size)

		ret := Move{
			OK: true,
			Colour: BLACK,
			X: x,
			Y: y,
		}

		if valid == false {
			ret.Pass = true
		}

		return ret
	}

	for _, foo := range self.Props["W"] {

		x, y, valid := point_from_string(foo, size)

		ret := Move{
			OK: true,
			Colour: WHITE,
			X: x,
			Y: y,
		}

		if valid == false {
			ret.Pass = true
		}

		return ret
	}

	return Move{OK: false}
}

// -------------------------------------------------------------------------

type App struct {
	Window			*sdl.Window
	Renderer		*sdl.Renderer
	PixelWidth		int32
	PixelHeight		int32
	CellWidth		int32
	Margin			int32
	Offset			int32
	SZ				int32
}


func NewApp(SZ, cell_width, margin int32) *App {

	self := new(App)

	self.SZ = SZ

	self.CellWidth = cell_width
	self.Offset = self.CellWidth / 2
	self.Margin = margin

	self.PixelWidth = (self.CellWidth * SZ) + (self.Margin * 2)
	self.PixelHeight = self.PixelWidth

	self.InitSDL()

	self.DrawGrid()
	self.Flip()

	return self
}


func (self *App) InitSDL() {

	fmt.Printf("Init...\n")

	err := sdl.Init(sdl.INIT_EVERYTHING)
	if err != nil {
		panic(err)
	}

	fmt.Printf("CreateWindow...\n")

	self.Window, err = sdl.CreateWindow(TITLE, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, self.PixelWidth, self.PixelHeight, sdl.WINDOW_SHOWN)
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
	self.Renderer.FillRect(&sdl.Rect{0, 0, self.PixelWidth, self.PixelHeight})
}


func (self *App) PixelXY(x, y int32) (int32, int32) {
	retx := x * self.CellWidth + self.Offset + self.Margin
	rety := y * self.CellWidth + self.Offset + self.Margin
	return retx, rety
}


func (self *App) AllHoshi() []Point {
	// FIXME - generalise
	return []Point{{3, 3}, {3, 9}, {3, 15}, {9, 3}, {9, 9}, {9, 15}, {15, 3}, {15, 9}, {15, 15}}
}


func (self *App) DrawGrid() {

	self.Cls(210, 175, 120)

	self.Renderer.SetDrawColor(0, 0, 0, 255)

	for x := int32(0) ; x < self.SZ ; x++ {
		x1, y1 := self.PixelXY(x, 0)
		x2, y2 := self.PixelXY(x, self.SZ - 1)
		self.Renderer.DrawLine(x1, y1, x2, y2)
	}

	for y := int32(0) ; y < self.SZ ; y++ {
		x1, y1 := self.PixelXY(0, y)
		x2, y2 := self.PixelXY(self.SZ - 1, y)
		self.Renderer.DrawLine(x1, y1, x2, y2)
	}

	for _, hoshi := range self.AllHoshi() {
		x, y := self.PixelXY(hoshi.X, hoshi.Y)
		self.Renderer.DrawRect(&sdl.Rect{x - 1, y - 1, 3, 3})
	}
}


func (self *App) Poll() {

	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {

		switch event.(type) {

		case *sdl.QuitEvent:
			self.Shutdown()
			os.Exit(0)
		}
	}
}

// -------------------------------------------------------------------------

func main() {
	app := NewApp(19, 24, 20)
	for {
		app.Poll()
	}
}

func point_from_string(s string, size int) (x int, y int, ok bool) {

	// If ok == false, that means the move was a pass.

	if len(s) < 2 {
		return 0, 0, false
	}

	x = int(s[0]) - 97
	y = int(s[1]) - 97

	ok = false

	if x >= 0 && x < size && y >= 0 && y < size {
		ok = true
	}

	return x, y, ok
}
