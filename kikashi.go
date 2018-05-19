package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/veandco/go-sdl2/sdl"
)

const TITLE = "Kikashi"
const ALPHA = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const DEFAULT_SIZE = 19

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

// -------------------------------------------------------------------------
// Nodes in an SGF tree...

type Node struct {
	Props			map[string][]string
	Children		[]*Node
	Parent			*Node
	Board			[][]Colour		// Created immediately by NewNode().
	SZ_cache		int32			// Cached value. 0 means not cached yet.
}


func NewNode(parent *Node, props map[string][]string) *Node {

	node := new(Node)
	node.Parent = parent
	node.Props = make(map[string][]string)

	for key, _ := range props {
		for _, s := range props[key] {
			node.Props[key] = append(node.Props[key], s)
		}
	}

	if node.Parent != nil {
		node.Parent.Children = append(node.Parent.Children, node)
	}

	node.make_board()
	return node
}


func new_bare_node(parent *Node) *Node {

	// Doesn't handle properties or make a board.
	// Used only for file loading.

	node := new(Node)
	node.Parent = parent
	node.Props = make(map[string][]string)

	if parent != nil {
		parent.Children = append(parent.Children, node)
	}

	return node
}


func (self *Node) AddValue(key, value string) {

	// Disallow things that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("AddValue(): Board altering properties disallowed after creation")
		}
	}

	self.add_value(key, value)
}


func (self *Node) add_value(key, value string) {			// Handles escaping

	value = escape_string(value)

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
			panic("SetValue(): Board altering properties disallowed after creation")
		}
	}

	self.Props[key] = nil
	self.add_value(key, value)
}


func (self *Node) GetValue(key string) (value string, ok bool) {

	// Get the value for the key, on the assumption that there's only 1 value.

	list := self.Props[key]

	if len(list) == 0 {
		return "", false
	}

	return unescape_string(list[0]), true
}


func (self *Node) MoveInfo() Move {

	sz := self.Size()

	// There should only be 1 move in a valid SGF node.

	for _, foo := range self.Props["B"] {

		x, y, valid := point_from_SGF_string(foo, sz)

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

		x, y, valid := point_from_SGF_string(foo, sz)

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

	if self.SZ_cache == 0 {

		// We don't have the info cached...

		if self.Parent == nil {

			// We are root...

			sz_string, ok := self.GetValue("SZ")

			if ok {

				val, err := strconv.Atoi(sz_string)

				if err == nil {

					if val > 0 && val <= 52 {
						self.SZ_cache = int32(val)
					}
				}
			}

			if self.SZ_cache == 0 {
				self.SZ_cache = DEFAULT_SIZE
				self.SetValue("SZ", fmt.Sprintf("%d", DEFAULT_SIZE))			// Set the actual property in the root.
			}

		} else {

			self.SZ_cache = self.Parent.Size()			// Recurse.

		}
	}

	return self.SZ_cache
}


func (self *Node) NextColour() Colour {

	// What colour a new move made from this node should be.
	// FIXME... should be better.

	if len(self.Props["B"]) > 0 {
		return WHITE
	} else if len(self.Props["W"]) > 0 {
		return BLACK
	} else if len(self.Props["AB"]) > 0 {
		return WHITE
	} else if len(self.Props["AW"]) > 0 {
		return BLACK
	} else {
		return BLACK
	}
}


func (self *Node) make_board() {

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
		x, y, ok := point_from_SGF_string(foo, sz)
		if ok { self.Board[x][y] = BLACK }
	}

	for _, foo := range self.Props["AW"] {
		x, y, ok := point_from_SGF_string(foo, sz)
		if ok { self.Board[x][y] = WHITE }
	}

	for _, foo := range self.Props["AE"] {
		x, y, ok := point_from_SGF_string(foo, sz)
		if ok { self.Board[x][y] = EMPTY }
	}

	// Play move: B / W

	for _, foo := range self.Props["B"] {
		x, y, ok := point_from_SGF_string(foo, sz)
		if ok { self.handle_move(BLACK, x, y) }
	}

	for _, foo := range self.Props["W"] {
		x, y, ok := point_from_SGF_string(foo, sz)
		if ok { self.handle_move(WHITE, x, y) }
	}
}


func (self *Node) make_board_recursive() {

	// Normally, new nodes have their board made instantly,
	// but not when loading a file, hence the need for this.

	self.make_board()
	for _, child := range self.Children {
		child.make_board_recursive()
	}
}


func (self *Node) handle_move(colour Colour, x, y int32) {

	// Additional changes to the board based on moves in the properties.
	// No legality checking here.

	if colour != BLACK && colour != WHITE {
		panic("handle_move(): colour != BLACK && colour != WHITE")
	}

	opponent := BLACK ; if colour == BLACK { opponent = WHITE }

	sz := self.Size()

	if x < 0 || x >= sz || y < 0 || y >= sz {
		panic("handle_move(): off board")
	}

	self.Board[x][y] = colour

	for _, point := range adjacent_points(x, y, sz) {
		if self.Board[point.X][point.Y] == opponent {
			if self.GroupHasLiberties(point.X, point.Y) == false {
				self.destroy_group(point.X, point.Y)
			}
		}
	}

	if self.GroupHasLiberties(x, y) == false {
		self.destroy_group(x, y)
	}
}


func (self *Node) GroupHasLiberties(x, y int32) bool {

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


func (self *Node) destroy_group(x, y int32) {

	colour := self.Board[x][y]
	if colour != BLACK && colour != WHITE {
		panic("destroy_group: colour != BLACK && colour != WHITE")
	}

	self.Board[x][y] = EMPTY

	for _, point := range adjacent_points(x, y, self.Size()) {
		if self.Board[point.X][point.Y] == colour {
			self.destroy_group(point.X, point.Y)
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
		self.RemoveChild(new_node)
		return nil, fmt.Errorf("TryMove(): Ko")
	}

	if new_node.Board[x][y] == EMPTY {
		self.RemoveChild(new_node)
		return nil, fmt.Errorf("TryMove(): Suicide")
	}

	return new_node, nil
}


func (self *Node) TryPass(colour Colour) *Node {

	if colour != BLACK && colour != WHITE {
		panic("TryMove(): colour != BLACK && colour != WHITE")
	}

	for _, child := range self.Children {
		mi := child.MoveInfo()
		if mi.OK && mi.Pass && mi.Colour == colour {
			return child
		}
	}

	key := "B" ; if colour == WHITE { key = "W" }
	new_node := NewNode(self, map[string][]string{key: []string{""}})
	return new_node
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

	if self == nil || other == nil || self.Board == nil || other.Board == nil {
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


func (self *Node) GetRoot() *Node {
	node := self
	for {
		if node.Parent != nil {
			node = node.Parent
		} else {
			return node
		}
	}
}


func (self *Node) GetEnd() *Node {
	node := self
	for {
		if len(node.Children) > 0 {
			node = node.Children[0]
		} else {
			return node
		}
	}
}


func (self *Node) Save(filename string) {

	outfile, _ := os.Create(filename)					// FIXME: handle errors!
	defer outfile.Close()

	w := bufio.NewWriter(outfile)						// bufio for speedier output if file is huge.
	defer w.Flush()

	self.GetRoot().WriteTree(w)
}


func (self *Node) WriteTree(outfile io.Writer) {		// Relies on values already being correctly backslash-escaped

	node := self

	fmt.Fprintf(outfile, "(")

	for {

		fmt.Fprintf(outfile, ";")

		for key, _ := range node.Props {

			fmt.Fprintf(outfile, "%s", key)

			for _, value := range node.Props[key] {
				fmt.Fprintf(outfile, "[%s]", value)
			}
		}

		if len(node.Children) > 1 {

			for _, child := range node.Children {
				child.WriteTree(outfile)
			}

			break

		} else if len(node.Children) == 1 {

			node = node.Children[0]
			continue

		} else {

			break

		}

	}

	fmt.Fprintf(outfile, ")\n")
	return
}


func (self *Node) FullGTP() []string {

	// Return a full list of GTP commands to recreate this position.
	// Doesn't work if "AE" properties are present anywhere in the line.

	var nodes []*Node
	var commands []string

	sz := self.Size()
	node := self

	// Make a list of relevant nodes, in reverse order...

	for {
		nodes = append(nodes, node)
		if node.Parent != nil {
			node = node.Parent
		} else {
			break
		}
	}

	commands = append(commands, fmt.Sprintf("boardsize %v", sz))
	commands = append(commands, "clear_board")

	for n := len(nodes) - 1; n >= 0; n-- {

		node = nodes[n]

		for _, foo := range node.Props["AB"] {
			x, y, ok := point_from_SGF_string(foo, sz)
			if ok {
				commands = append(commands, fmt.Sprintf("play B %v", human_string_from_point(x, y, sz)))
			}
		}

		for _, foo := range node.Props["AW"] {
			x, y, ok := point_from_SGF_string(foo, sz)
			if ok {
				commands = append(commands, fmt.Sprintf("play W %v", human_string_from_point(x, y, sz)))
			}
		}

		// Play move: B / W

		for _, foo := range node.Props["B"] {
			x, y, ok := point_from_SGF_string(foo, sz)
			if ok {
				commands = append(commands, fmt.Sprintf("play B %v", human_string_from_point(x, y, sz)))
			}
		}

		for _, foo := range node.Props["W"] {
			x, y, ok := point_from_SGF_string(foo, sz)
			if ok {
				commands = append(commands, fmt.Sprintf("play W %v", human_string_from_point(x, y, sz)))
			}
		}
	}

	return commands
}

// -------------------------------------------------------------------------

func LoadFile(filename string) (*Node, error) {

	sgf_bytes, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	root, err := load_sgf(string(sgf_bytes))

	if err != nil {
		return nil, err
	}

	// So we now have a tree but without boards...

	root.make_board_recursive()
	return root, nil
}


func load_sgf_tree(sgf string, parent_of_local_root *Node) (*Node, int, error) {

	// FIXME: this is not unicode aware. Potential problems exist
	// if a unicode code point contains a meaningful character.

	var root *Node
	var node *Node

	var inside bool
	var value string
	var key string
	var keycomplete bool
	var chars_to_skip int

	var err error

	for i := 0; i < len(sgf); i++ {

		c := sgf[i]

		if chars_to_skip > 0 {
			chars_to_skip--
			continue
		}

		if inside {

			if c == '\\' {
				if len(sgf) <= i + 1 {
					return nil, 0, fmt.Errorf("load_sgf_tree: escape character at end of input")
				}
				value += string('\\')
				value += string(sgf[i + 1])
				chars_to_skip = 1
			} else if c == ']' {
				inside = false
				if node == nil {
					return nil, 0, fmt.Errorf("load_sgf_tree: node == nil after: else if c == ']'")
				}
				node.add_value(key, value)
			} else {
				value += string(c)
			}

		} else {

			if c == '[' {
				value = ""
				inside = true
				keycomplete = true
			} else if c == '(' {
				if node == nil {
					return nil, 0, fmt.Errorf("load_sgf_tree: node == nil after: else if c == '('")
				}
				_, chars_to_skip, err = load_sgf_tree(sgf[i + 1:], node)
				if err != nil {
					return nil, 0, err
				}
			} else if c == ')' {
				if root == nil {
					return nil, 0, fmt.Errorf("load_sgf_tree: root == nil after: else if c == ')'")
				}
				return root, i + 1, nil		// Return characters read.
			} else if c == ';' {
				if node == nil {
					newnode := new_bare_node(parent_of_local_root)
					root = newnode
					node = newnode
				} else {
					newnode := new_bare_node(node)
					node = newnode
				}
			} else {
				if c >= 'A' && c <= 'Z' {
					if keycomplete {
						key = ""
						keycomplete = false
					}
					key += string(c)
				}
			}
		}
	}

	if root == nil {
		return nil, 0, fmt.Errorf("load_sgf_tree: root == nil at function end")
	}

	return root, len(sgf), nil		// Return characters read.
}


func load_sgf(sgf string) (*Node, error) {

	sgf = strings.TrimSpace(sgf)
	if sgf[0] == '(' {				// the load_sgf_tree() function assumes the
		sgf = sgf[1:]				// leading "(" has already been discarded.
	}

	root, _, err := load_sgf_tree(sgf, nil)
	return root, err
}

// -------------------------------------------------------------------------

type App struct {
	Window				*sdl.Window
	Renderer			*sdl.Renderer
	PixelWidth			int32
	PixelHeight			int32
	CellWidth			int32
	Margin				int32
	Offset				int32

	Node				*Node

	LZ_Stdin			io.Writer
	LZ_Stdout_Buffer	LineBuffer
	LZ_Stderr_Buffer	LineBuffer

	Variations			map[Point][]Move
	VariationsNext		map[Point][]Move
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

	self.Variations = make(map[Point][]Move)
	self.VariationsNext = make(map[Point][]Move)

	exec_command := exec.Command("./leelaz.exe", "--gtp", "-w", "network")

	self.LZ_Stdin, _ = exec_command.StdinPipe()
	self.LZ_Stdout_Buffer.Reader, _ = exec_command.StdoutPipe()
	self.LZ_Stderr_Buffer.Reader, _ = exec_command.StderrPipe()

	exec_command.Start()

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
		TITLE, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, self.PixelWidth, self.PixelHeight, sdl.WINDOW_SHOWN)
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


func (self *App) BoardXY(x1, y1 int32) (int32, int32) {

	min := self.Offset + self.Margin - (self.CellWidth / 2)
	max := ((self.Node.Size() - 1) * self.CellWidth) + self.Offset + self.Margin + (self.CellWidth / 2)

	diff := float64(max - min)

	retx_f := (float64(x1 - min) / diff) * float64(self.Node.Size())
	rety_f := (float64(y1 - min) / diff) * float64(self.Node.Size())

	retx := int32(math.Floor(retx_f))
	rety := int32(math.Floor(rety_f))

	if retx < 0 { retx = 0 }
	if retx >= self.Node.Size() { retx = self.Node.Size() - 1 }
	if rety < 0 { rety = 0 }
	if rety >= self.Node.Size() { rety = self.Node.Size() - 1 }

	return retx, rety
}


func (self *App) AllHoshi() []Point {
	// FIXME - generalise
	return []Point{{3, 3}, {3, 9}, {3, 15}, {9, 3}, {9, 9}, {9, 15}, {15, 3}, {15, 9}, {15, 15}}
}


func (self *App) DrawGrid() {

	self.Cls(208, 172, 114)

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


func (self *App) DrawBoard() {

	self.DrawGrid()

	for x := int32(0); x < self.Node.Size(); x++ {

		for y := int32(0); y < self.Node.Size(); y++ {

			if self.Node.Board[x][y] != EMPTY {

				x1, y1 := self.PixelXY(x, y)

				if self.Node.Board[x][y] == BLACK {
					self.Fcircle(x1, y1, self.CellWidth / 2, 0, 0, 0)
				} else if self.Node.Board[x][y] == WHITE {
					self.Fcircle(x1, y1, self.CellWidth / 2, 255, 255, 255)
					self.Circle(x1, y1, self.CellWidth / 2, 0, 0, 0)
				}
			}
		}
	}

	move_info := self.Node.MoveInfo()

	if move_info.OK && move_info.Pass == false {
		x1, y1 := self.PixelXY(move_info.X, move_info.Y)
		if self.Node.Board[move_info.X][move_info.Y] == WHITE {
			self.Fcircle(x1, y1, self.CellWidth / 8, 0, 0, 0)
		} else {
			self.Fcircle(x1, y1, self.CellWidth / 8, 255, 255, 255)
		}
	}
}


func (self *App) Circle(x, y, radius int32, r, g, b uint8) {

	self.Renderer.SetDrawColor(r, g, b, 255)

	var pyth float64
	var topline bool = true
	var lastiplusone int32

	for j := radius - 1; j >= 0; j-- {
		for i := radius - 1; i >= 0; i-- {
			pyth = math.Sqrt(math.Pow(float64(i), 2) + math.Pow(float64(j), 2))
			if (pyth < float64(radius) - 0.5) {
				if topline {                    // i.e. if we're on the top (and, with mirroring, bottom) lines
					topline = false
					self.Renderer.DrawLine(x - i - 1, y - j - 1, x + i, y - j - 1)
					self.Renderer.DrawLine(x - i - 1, y + j, x + i, y + j)
					lastiplusone = i + 1
				} else {
					if lastiplusone == i + 1 {
						self.Renderer.DrawPoint(x - i - 1, y - j - 1)
						self.Renderer.DrawPoint(x + i, y - j - 1)
						self.Renderer.DrawPoint(x - i - 1, y + j)
						self.Renderer.DrawPoint(x + i, y + j)
					} else {
						self.Renderer.DrawLine(x - i - 1, y - j - 1, x - lastiplusone - 1, y - j - 1)
						self.Renderer.DrawLine(x + lastiplusone, y - j - 1, x + i, y - j - 1)
						self.Renderer.DrawLine(x - i - 1, y + j, x - lastiplusone - 1, y + j)
						self.Renderer.DrawLine(x + lastiplusone, y + j, x + i, y + j)
						lastiplusone = i + 1
					}
				}
				break
			}
		}
	}
}


func (self *App) Fcircle(x, y, radius int32, r, g, b uint8) {

	var pyth float64;

	self.Renderer.SetDrawColor(r, g, b, 255)

	for j := radius; j >= 0; j-- {
		for i:= radius; i >= 0; i-- {
			pyth = math.Sqrt(math.Pow(float64(i), 2) + math.Pow(float64(j), 2));
			if (pyth < float64(radius) - 0.5) {
				self.Renderer.DrawLine(x - i - 1, y - j - 1, x + i, y - j - 1)
				self.Renderer.DrawLine(x - i - 1, y + j, x + i, y + j)
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

				x, y := self.BoardXY(event.X, event.Y)

				new_node, err := self.Node.TryMove(self.Node.NextColour(), x, y)
				if err != nil {
					// fmt.Printf("%v\n", err)
				} else {
					self.Node = new_node
					self.DrawBoard()
				}
			}

		case *sdl.MouseWheelEvent:

			if event.Y > 0 {

				if self.Node.Parent != nil {

					self.Node = self.Node.Parent
					self.DrawBoard()
				}
			}

			if event.Y < 0 {		// Ideally we'd remember which line of the game we've been in.

				if len(self.Node.Children) > 0 {

					self.Node = self.Node.Children[0]
					self.DrawBoard()
				}
			}

		case *sdl.KeyboardEvent:

			if event.Type == sdl.KEYDOWN {

				switch event.Keysym.Sym {

				// https://github.com/veandco/go-sdl2/blob/master/sdl/keycode.go

				case sdl.K_s:

					var filename string
					var dialog_done bool

					result_chan := make(chan string)
					go file_dialog(true, result_chan)

					// SDL is healthier when it's constantly getting polled.
					// Therefore, while we await the dialog, continue to poll
					// for events (but just ignore them).

					for !dialog_done {

						select {

						case filename = <- result_chan:
							dialog_done = true

						default:
							self.PollNoAction()
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

					// SDL is healthier when it's constantly getting polled.
					// Therefore, while we await the dialog, continue to poll
					// for events (but just ignore most of them).

					for !dialog_done {

						select {

						case filename = <- result_chan:
							dialog_done = true

						default:
							self.PollNoAction()
						}
					}

					if filename == "" {
						break
					}

					new_root, err := LoadFile(string(filename))
					if err != nil {
						fmt.Printf("%v\n", err)
						break
					}

					self.Node = new_root
					self.DrawBoard()

				case sdl.K_p:

					self.Node = self.Node.TryPass(self.Node.NextColour())
					self.DrawBoard()

				case sdl.K_END:

					self.Node = self.Node.GetEnd()
					self.DrawBoard()

				case sdl.K_HOME:

					self.Node = self.Node.GetRoot()
					self.DrawBoard()

				case sdl.K_DOWN:

					if len(self.Node.Children) > 0 {
						self.Node = self.Node.Children[0]
						self.DrawBoard()
					}

				case sdl.K_UP:

					if self.Node.Parent != nil {
						self.Node = self.Node.Parent
						self.DrawBoard()
					}
				}
			}
		}
	}
}


func (self *App) GetNewStderrLines() []string {

	var new_lines []string

	self.LZ_Stderr_Buffer.Lock()
	for _, line := range self.LZ_Stderr_Buffer.Lines {
		new_lines = append(new_lines, line)
	}
	self.LZ_Stderr_Buffer.Lines = nil
	self.LZ_Stderr_Buffer.Unlock()

	return new_lines
}


func (self *App) Analyse() {

	new_lines := self.GetNewStderrLines()

	for _, line := range new_lines {

		if line == "~end" {
			self.Variations = self.VariationsNext
			self.VariationsNext = make(map[Point][]Move)
		}

		fmt.Printf("%s\n", line)
	}
}


func (self *App) Run() {
	self.LZ_Stdin.Write([]byte("time_left B 0 0\n"))
	for {
		self.Poll()
		self.Analyse()
		self.Flip()
		// FIXME: some sleep
		// FIXME: consume stdout
	}
}

// -------------------------------------------------------------------------

func main() {
	app := NewApp(DEFAULT_SIZE, 36, 20)
	app.Run()
}


func point_from_SGF_string(s string, size int32) (x int32, y int32, ok bool) {

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


func human_string_from_point(x, y, size int32) string {
	const letters = "ABCDEFGHJKLMNOPQRSTUVWXYZ"
	return fmt.Sprintf("%c%v", letters[x], size - y)
}


func point_from_human_string(s string, size int32) (x int32, y int32, ok bool) {

	if len(s) < 2 || len(s) > 3 {
		return 0, 0, false
	}

	if s[0] < 'A' || s[0] > 'Z' {
		return 0, 0, false
	}

	if s[1] < '0' || s[1] > '9' {
		return 0, 0, false
	}

	if len(s) == 3 && (s[2] < '0' || s[2] > '9') {
		return 0, 0, false
	}

	x = int32(s[0]) - 65
	if x >= 8 {
		x--
	}

	y_int, _ := strconv.Atoi(s[1:])
	y = size - int32(y_int)

	return x, y, true
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


func escape_string(s string) string {

	// Treating the input as a byte sequence, not a sequence of code points. Meh.

	var new_s []byte

	for n := 0; n < len(s); n++ {
		if s[n] == '\\' || s[n] == ']' {
			new_s = append(new_s, '\\')
		}
		new_s = append(new_s, s[n])
	}

	return string(new_s)
}


func unescape_string(s string) string {

	// Treating the input as a byte sequence, not a sequence of code points. Meh.
	// Some issues with unicode.

	var new_s []byte

	forced_accept := false

	for n := 0; n < len(s); n++ {

		if forced_accept {
			new_s = append(new_s, s[n])
			forced_accept = false
			continue
		}

		if s[n] == '\\' {
			forced_accept = true
			continue
		}

		new_s = append(new_s, s[n])
	}

	return string(new_s)
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


func opposite_colour(c Colour) Colour {
	if c == EMPTY {
		return EMPTY
	} else if c == BLACK {
		return WHITE
	} else {
		return BLACK
	}
}


func pv_from_line(s string, next_colour Colour, size int32) []Move {

	tokens := strings.Fields(s)

	if len(tokens) < 9 || tokens[7] != "PV:" {
		return nil
	}

	var moves []Move

	for _, t := range tokens[8:] {

		var mv Move

		if t == "pass" {

			mv = Move{
				OK: true,
				Pass: true,
				Colour: next_colour,
			}

		} else {

			x, y, ok := point_from_human_string(t, size)

			if ok {
				mv = Move{
					OK: true,
					Colour: next_colour,
					X: x,
					Y: y,
				}
			} else {
				fmt.Printf("Warning: point_from_human_string() returned ok: false")
			}
		}

		if mv.OK {
			moves = append(moves, mv)
			next_colour = opposite_colour(next_colour)
		}
	}

	return moves
}
