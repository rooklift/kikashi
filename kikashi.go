package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/veandco/go-sdl2/sdl"
)

const TITLE = "Kikashi"
const ALPHA = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var MUTORS = []string{"B", "W", "AB", "AW", "AE"}

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

type Node struct {
	Props			map[string][]string
	Children		[]*Node
	Parent			*Node
	Board			[][]Colour		// Created immediately by NewNode()
	__SZ			int32			// Cached value. 0 means not cached yet.
}


func NewNode(parent *Node, props map[string][]string) *Node {

	node := new(Node)
	node.Parent = parent
	node.Props = make(map[string][]string)

	for key, _ := range props {
		node.Props[key] = nil
		for _, s := range props[key] {
			node.Props[key] = append(node.Props[key], s)
		}
	}

	if node.Parent != nil {
		node.Parent.Children = append(node.Parent.Children, node)
	}

	node.__make_board()
	return node
}


// FIXME: Add and Set need escaping...


func (self *Node) AddValue(key, value string) {

	// Disallow things that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("AddValue(): Already have a blocking key")
		}
	}

	for i := 0; i < len(self.Props[key]); i++ {
		if self.Props[key][i] == value {
			return
		}
	}

	self.Props[key] = append(self.Props[key], value)
}


func (self *Node) SetValue(key, value string) {

	// Disallow things that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("SetValue(): Already have a blocking key")
		}
	}

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

	// Note that this line is NOT a check of the property SZ:

	if self.__SZ == 0 {

		// We don't have the info cached...

		if self.Parent == nil {

			// We are root...

			sz_string, ok := self.GetValue("SZ")

			if ok {

				val, err := strconv.Atoi(sz_string)

				if err == nil {

					if val > 0 && val <= 52 {
						self.__SZ = int32(val)
					}
				}
			}

			if self.__SZ == 0 {
				self.__SZ = 19
				self.SetValue("SZ", "19")			// Set the actual property in the root.
			}

		} else {

			self.__SZ = self.Parent.Size()			// Recurse.

		}
	}

	return self.__SZ
}


func (self *Node) __make_board() {

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
		if ok { self.__handle_move(BLACK, x, y) }
	}

	for _, foo := range self.Props["W"] {
		x, y, ok := point_from_string(foo, sz)
		if ok { self.__handle_move(WHITE, x, y) }
	}
}


func (self *Node) __handle_move(colour Colour, x, y int32) {

	// Additional changes to the board based on moves in the properties.
	// No legality checking here.

	if colour != BLACK && colour != WHITE {
		panic("__handle_move(): colour != BLACK && colour != WHITE")
	}

	opponent := BLACK ; if colour == BLACK { opponent = WHITE }

	sz := self.Size()

	if x < 0 || x >= sz || y < 0 || y >= sz {
		panic("__handle_move(): off board")
	}

	self.Board[x][y] = colour

	for _, point := range adjacent_points(x, y, sz) {
		if self.Board[point.X][point.Y] == opponent {
			if self.GroupHasLiberties(point.X, point.Y) == false {
				self.__destroy_group(point.X, point.Y)
			}
		}
	}

	if self.GroupHasLiberties(x, y) == false {
		self.__destroy_group(x, y)
	}
}


func (self *Node) GroupHasLiberties(x, y int32) bool {

	touched := make(map[Point]bool)
	return self.__group_has_liberties(x, y, touched)
}


func (self *Node) __group_has_liberties(x, y int32, touched map[Point]bool) bool {

	touched[Point{x, y}] = true

	colour := self.Board[x][y]
	if colour != BLACK && colour != WHITE {
		panic("__group_has_liberties(): colour != BLACK && colour != WHITE")
	}

	for _, point := range adjacent_points(x, y, self.Size()) {
		if self.Board[point.X][point.Y] == EMPTY {
			return true
		} else if self.Board[point.X][point.Y] == colour {
			if touched[Point{point.X, point.Y}] == false {
				if self.__group_has_liberties(point.X, point.Y, touched) {
					return true
				}
			}
		}
	}

	return false
}


func (self *Node) __destroy_group(x, y int32) {

	colour := self.Board[x][y]
	if colour != BLACK && colour != WHITE {
		panic("__destroy_group: colour != BLACK && colour != WHITE")
	}

	self.Board[x][y] = EMPTY

	for _, point := range adjacent_points(x, y, self.Size()) {
		if self.Board[point.X][point.Y] == colour {
			self.__destroy_group(point.X, point.Y)
		}
	}
}


func (self *Node) TryMove(colour Colour, x, y int32) (*Node, error) {

	// Returns a new node on success.

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

	// If the move already exists, just return the (first) relevant child...

	mv := Move{
		OK: true,
		Pass: false,
		Colour: colour,
		X: x,
		Y: y,
	}

	for _, child := range self.Children {
		if child.MoveInfo() == mv {
			return child, nil
		}
	}

	// Legality checks...

	key := "B" ; if colour == WHITE { key = "W" }
	val := string(ALPHA[x]) + string(ALPHA[y])

	new_node := NewNode(self, map[string][]string{key: []string{val}})

	if new_node.SameBoard(self.Parent) {
		self.Parent.RemoveChild(new_node)
		return nil, fmt.Errorf("TryMove(): Ko")
	}

	if new_node.Board[x][y] == EMPTY {
		self.Parent.RemoveChild(new_node)
		return nil, fmt.Errorf("TryMove(): Suicide")
	}

	return new_node, nil
}


func (self *Node) RemoveChild(child *Node) {

	if self == nil {
		return
	}

	for i := len(self.Children) - 1; i >= 0; i-- {
		if self.Children[i] == child {
			self.Children = append(self.Children[:i], self.Children[i+1:]...)
		}
	}
}


func (self *Node) SameBoard(other *Node) bool {

	if self.Board == nil || other.Board == nil {
		return false
	}

	sz := self.Size()

	for x := int32(0); x < sz; x++ {
		for y := int32(0); y < sz; y++ {
			if self.Board[x][y] != other.Board[x][y] {
				return false
			}
		}
	}

	return true
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
	Node			*Node
}


func NewApp(SZ, cell_width, margin int32) *App {

	self := new(App)
	self.Node = NewNode(nil, nil)

	self.CellWidth = cell_width
	self.Offset = self.CellWidth / 2
	self.Margin = margin

	self.PixelWidth = (self.CellWidth * self.Node.Size()) + (self.Margin * 2)
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

	for x := int32(0) ; x < self.Node.Size() ; x++ {
		x1, y1 := self.PixelXY(x, 0)
		x2, y2 := self.PixelXY(x, self.Node.Size() - 1)
		self.Renderer.DrawLine(x1, y1, x2, y2)
	}

	for y := int32(0) ; y < self.Node.Size() ; y++ {
		x1, y1 := self.PixelXY(0, y)
		x2, y2 := self.PixelXY(self.Node.Size() - 1, y)
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
