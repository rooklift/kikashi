package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/veandco/go-sdl2/sdl"
)

const TITLE = "Kikashi"
const ALPHA = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

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
	X				int32
	Y				int32
}

// -------------------------------------------------------------------------
// Nodes in an SGF tree...
//
// After creation, any B, W, AB, AW, AE tags should be added,
// and then MakeBoard() should be called.

type Node struct {
	Props			map[string][]string
	Children		[]*Node
	Parent			*Node
	Board			[][]Colour		// Shouldn't be left as nil for long.
	SZ				int32			// 0 means we haven't got it cached here.
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


// FIXME: Add and Set need escaping...


func (self *Node) AddValue(key, value string) {

	for i := 0; i < len(self.Props[key]); i++ {
		if self.Props[key][i] == value {
			return
		}
	}

	self.Props[key] = append(self.Props[key], value)
}


func (self *Node) SetValue(key, value string) {
	self.Props[key] = nil
	self.AddValue(key, value)
}


func (self *Node) GetValue(key string) (value string, ok bool) {

	// Get the value for the key, on the assumption that there's only 1 value.

	list := self.Props[key]

	if len(list) == 0 {
		return "", false
	}

	return list[0], true
}


func (self *Node) MoveInfo() Move {

	sz := self.Size()

	// There should only be 1 move in a valid SGF node.

	for _, foo := range self.Props["B"] {

		x, y, valid := point_from_string(foo, sz)

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

		x, y, valid := point_from_string(foo, sz)

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


func (self *Node) Size() int32 {

	// Note that this is NOT a check of the property SZ...

	if self.SZ == 0 {

		// We don't have the info cached...

		if self.Parent == nil {

			// We are root...

			sz_string, ok := self.GetValue("SZ")

			if ok {

				val, err := strconv.Atoi(sz_string)

				if err == nil {

					if val > 0 && val <= 52 {
						self.SZ = int32(val)
					}
				}
			}

			if self.SZ == 0 {
				self.SZ = 19
				self.SetValue("SZ", "19")			// Set the actual property in the root.
			}

		} else {

			self.SZ = self.Parent.Size()			// Recurse.

		}
	}

	return self.SZ
}


func (self *Node) MakeBoard() {

	sz := self.Size()

	self.Board = make([][]Colour, sz)
	for x := 0; x < len(self.Board); x++ {
		self.Board[x] = make([]Colour, sz)
	}

	if self.Parent != nil {
		for x := int32(0); x < sz; x++ {
			for y := int32(0); y < sz; y++ {
				self.Board[x][y] = self.Parent.Board[x][y]
			}
		}
	}

	// Now fix the board using the properties...

	for _, foo := range self.Props["AB"] {
		x, y, ok := point_from_string(foo, sz)
		if ok { self.Board[x][y] = BLACK }
	}

	for _, foo := range self.Props["AW"] {
		x, y, ok := point_from_string(foo, sz)
		if ok { self.Board[x][y] = WHITE }
	}

	for _, foo := range self.Props["AE"] {
		x, y, ok := point_from_string(foo, sz)
		if ok { self.Board[x][y] = EMPTY }
	}

	// Play move: B / W

	for _, foo := range self.Props["B"] {
		x, y, ok := point_from_string(foo, sz)
		if ok { self.ForceMove(BLACK, x, y) }
	}

	for _, foo := range self.Props["W"] {
		x, y, ok := point_from_string(foo, sz)
		if ok { self.ForceMove(WHITE, x, y) }
	}
}


func (self *Node) ForceMove(colour Colour, x, y int32) {		// No legality checking here.

	if self.Board == nil {
		panic("ForceMove(): self.Board == nil")
	}

	if colour != BLACK && colour != WHITE {
		panic("ForceMove(): colour != BLACK && colour != WHITE")
	}

	opponent := BLACK ; if colour == BLACK { opponent = WHITE }

	sz := self.Size()

	if x < 0 || x >= sz || y < 0 || y >= sz {
		panic("ForceMove(): off board")
	}

	self.Board[x][y] = colour

	for _, point := range adjacent_points(x, y, sz) {
		if self.Board[point.X][point.Y] == opponent {
			if self.GroupHasLiberties(point.X, point.Y) == false {
				self.DestroyGroup(point.X, point.Y)
			}
		}
	}

	if self.GroupHasLiberties(x, y) == false {
		self.DestroyGroup(x, y)
	}
}


func (self *Node) GroupHasLiberties(x, y int32) bool {

	if self.Board == nil {
		panic("GroupHasLiberties(): self.Board == nil")
	}

	touched := make(map[Point]bool)
	return self.group_has_liberties(x, y, touched)
}


func (self *Node) group_has_liberties(x, y int32, touched map[Point]bool) bool {

	touched[Point{x, y}] = true

	colour := self.Board[x][y]
	if colour != BLACK && colour != WHITE {
		panic("group_has_liberties(): colour != BLACK && colour != WHITE")
	}

	for _, point := range adjacent_points(x, y, self.Size()) {
		if self.Board[point.X][point.Y] == EMPTY {
			return true
		} else if self.Board[point.X][point.Y] == colour {
			if touched[Point{point.X, point.Y}] == false {
				if self.group_has_liberties(point.X, point.Y, touched) {
					return true
				}
			}
		}
	}

	return false
}


func (self *Node) DestroyGroup(x, y int32) {

	if self.Board == nil {
		panic("DestroyGroup(): self.Board == nil")
	}

	colour := self.Board[x][y]
	if colour != BLACK && colour != WHITE {
		panic("DestroyGroup: colour != BLACK && colour != WHITE")
	}

	self.Board[x][y] = EMPTY

	for _, point := range adjacent_points(x, y, self.Size()) {
		if self.Board[point.X][point.Y] == colour {
			self.DestroyGroup(point.X, point.Y)
		}
	}
}


func (self *Node) TryMove(colour Colour, x, y int32) (*Node, error) {

	if self.Board == nil {
		panic("TryMove(): self.Board == nil")
	}

	if colour != BLACK && colour != WHITE {
		panic("TryMove(): colour != BLACK && colour != WHITE")
	}

	sz := self.Size()

	if x < 0 || x >= sz || y < 0 || y >= sz {
		return nil, fmt.Errorf("TryMove(): Off board")
	}
	if self.Board[x][y] != EMPTY {
		return nil, fmt.Errorf("TryMove(): Occupied point")
	}

	key := "B" ; if colour == WHITE { key = "W" }
	val := string(ALPHA[x]) + string(ALPHA[y])

	// If the move already exists, just return the (first) relevant child...

	test := Move{
		OK: true,
		Pass: false,
		Colour: colour,
		X: x,
		Y: y,
	}

	for _, child := range self.Children {
		if child.MoveInfo() == test {
			return child, nil
		}
	}

	// FIXME: Check for legality...

	new_node := NewNode(self)
	new_node.SetValue(key, val)
	new_node.MakeBoard()

	return new_node, nil
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
	Node			*Node
}


func NewApp(SZ, cell_width, margin int32) *App {

	self := new(App)

	self.SZ = SZ

	self.CellWidth = cell_width
	self.Offset = self.CellWidth / 2
	self.Margin = margin

	self.PixelWidth = (self.CellWidth * SZ) + (self.Margin * 2)
	self.PixelHeight = self.PixelWidth

	self.Node = NewNode(nil)

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

func point_from_string(s string, size int32) (x int32, y int32, ok bool) {

	// If ok == false, that means the move was a pass.

	if len(s) < 2 {
		return 0, 0, false
	}

	x = int32(s[0]) - 97
	y = int32(s[1]) - 97

	ok = false

	if x >= 0 && x < size && y >= 0 && y < size {
		ok = true
	}

	return x, y, ok
}

func adjacent_points(x, y, size int32) []Point {

	var ret []Point

	possibles := []Point{
		Point{x - 1, y},
		Point{x + 1, y},
		Point{x, y - 1},
		Point{x, y + 1},
	}

	for _, point := range possibles {
		if point.X >= 0 && point.X < size {
			if point.Y >= 0 && point.Y < size {
				ret = append(ret, point)
			}
		}
	}

	return ret
}
