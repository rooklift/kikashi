package kikashi

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// -------------------------------------------------------------------------

type Point struct {
	X				int32
	Y				int32
}

const DEFAULT_SIZE = 19
const ALPHA = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var MUTORS = []string{"B", "W", "AB", "AW", "AE"}

// -------------------------------------------------------------------------

type Colour int

const (
	EMPTY = Colour(iota)
	BLACK
	WHITE
)

var COLMAP = map[Colour]string{
	EMPTY: "?",
	BLACK: "B",
	WHITE: "W",
}

func (c Colour) Opposite() Colour {

	if c == EMPTY {
		return EMPTY
	} else if c == BLACK {
		return WHITE
	} else {
		return BLACK
	}
}

// -------------------------------------------------------------------------

type Move struct {
	OK				bool
	Pass			bool
	Colour			Colour
	X				int32
	Y				int32
	Size			int32
}

func (self *Move) String() string {

	if self.OK == false {
		return "(none.)"
	}

	if self.Pass {
		return fmt.Sprintf("(%spass)", COLMAP[self.Colour])
	}

	hs := HumanStringFromPoint(self.X, self.Y, self.Size)
	if len(hs) == 2 {
		hs += " "
	}

	return fmt.Sprintf("(%s %s)", COLMAP[self.Colour], hs)
}

// -------------------------------------------------------------------------

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


func NewTree(size int32) *Node {

	// Sizes over 25 are not recommended. But 52 is the hard limit for SGF.

	if size < 1 || size > 52 {
		panic(fmt.Sprintf("NewTree(): invalid size %v", size))
	}

	properties := make(map[string][]string)
	size_string := fmt.Sprintf("%d", size)
	properties["SZ"] = []string{size_string}
	properties["GM"] = []string{"1"}
	properties["FF"] = []string{"4"}

	return NewNode(nil, properties)
}


func new_bare_node(parent *Node) *Node {

	// Doesn't accept properties or make a board.
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

	// Disallow keys that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("AddValue(): Can't change board-altering properties")
		}
	}

	self.add_value(key, value)
}


func (self *Node) add_value(key, value string) {			// Handles escaping

	value = escape_string(value)

	for i := 0; i < len(self.Props[key]); i++ {				// Ignore if the value already exists
		if self.Props[key][i] == value {
			return
		}
	}

	self.Props[key] = append(self.Props[key], value)
}


func (self *Node) SetValue(key, value string) {

	// Disallow keys that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("SetValue(): Can't change board-altering properties")
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


func (self *Node) AllValues(key string) []string {

	// Return all values for the key, possibly zero

	list := self.Props[key]

	if len(list) == 0 {
		return nil
	}

	var ret []string

	for _, s := range list {
		ret = append(ret, unescape_string(s))
	}

	return ret
}


func (self *Node) DeleteValue(key, value string) {

	// Disallow keys that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("DeleteValue(): Can't change board-altering properties")
		}
	}

	for i := len(self.Props[key]) - 1; i >= 0; i-- {
		v := self.Props[key][i]
		if v == value {
			self.Props[key] = append(self.Props[key][:i], self.Props[key][i+1:]...)
		}
	}
}


func (self *Node) DeleteKey(key string) {

	// Disallow keys that change the board...

	for _, s := range MUTORS {
		if key == s {
			panic("DeleteKey(): Can't change board-altering properties")
		}
	}

	delete(self.Props, key)
}


func (self *Node) MoveInfo() Move {

	sz := self.Size()

	// There should only be 1 move in a valid SGF node.

	for _, foo := range self.Props["B"] {

		x, y, valid := PointFromSGFString(foo, sz)

		ret := Move{
			OK: true,
			Colour: BLACK,
			X: x,
			Y: y,
			Size: sz,
		}

		if valid == false {
			ret.Pass = true
		}

		return ret
	}

	for _, foo := range self.Props["W"] {

		x, y, valid := PointFromSGFString(foo, sz)

		ret := Move{
			OK: true,
			Colour: WHITE,
			X: x,
			Y: y,
			Size: sz,
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
		x, y, ok := PointFromSGFString(foo, sz)
		if ok { self.Board[x][y] = BLACK }
	}

	for _, foo := range self.Props["AW"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok { self.Board[x][y] = WHITE }
	}

	for _, foo := range self.Props["AE"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok { self.Board[x][y] = EMPTY }
	}

	// Play move: B / W

	for _, foo := range self.Props["B"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok { self.handle_move(BLACK, x, y) }
	}

	for _, foo := range self.Props["W"] {
		x, y, ok := PointFromSGFString(foo, sz)
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
	// On failure, returns the original node.

	if colour != BLACK && colour != WHITE {
		panic("TryMove(): colour != BLACK && colour != WHITE")
	}

	sz := self.Size()

	if x < 0 || x >= sz || y < 0 || y >= sz {
		return self, fmt.Errorf("TryMove(): Off board")
	}
	if self.Board[x][y] != EMPTY {
		return self, fmt.Errorf("TryMove(): Occupied point")
	}

	// If the move already exists, just return the (first) relevant child...

	mv := Move{
		OK: true,
		Pass: false,
		Colour: colour,
		X: x,
		Y: y,
		Size: sz,
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
		return self, fmt.Errorf("TryMove(): Ko")
	}

	if new_node.Board[x][y] == EMPTY {
		self.RemoveChild(new_node)
		return self, fmt.Errorf("TryMove(): Suicide")
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


func (self *Node) StepGTP() []string {

	// Return list of GTP commands to get to this position from the parent.

	var commands []string

	sz := self.Size()

	for _, foo := range self.Props["AB"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok {
			commands = append(commands, fmt.Sprintf("play B %v", HumanStringFromPoint(x, y, sz)))
		}
	}

	for _, foo := range self.Props["AW"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok {
			commands = append(commands, fmt.Sprintf("play W %v", HumanStringFromPoint(x, y, sz)))
		}
	}

	for _, foo := range self.Props["B"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok {
			commands = append(commands, fmt.Sprintf("play B %v", HumanStringFromPoint(x, y, sz)))
		} else {
			commands = append(commands, fmt.Sprintf("play B pass"))
		}
	}

	for _, foo := range self.Props["W"] {
		x, y, ok := PointFromSGFString(foo, sz)
		if ok {
			commands = append(commands, fmt.Sprintf("play W %v", HumanStringFromPoint(x, y, sz)))
		} else {
			commands = append(commands, fmt.Sprintf("play W pass"))
		}
	}

	return commands
}


func (self *Node) FullGTP() []string {

	// Return a full list of GTP commands to recreate this position.
	// Doesn't work if "AE" properties are present anywhere in the line.

	var commands []string
	var nodes []*Node

	// Make a list of relevant nodes, in reverse order...

	node := self

	for {
		nodes = append(nodes, node)
		if node.Parent != nil {
			node = node.Parent
		} else {
			break
		}
	}

	commands = append(commands, fmt.Sprintf("boardsize %v", node.Size()))
	commands = append(commands, "clear_board")

	for n := len(nodes) - 1; n >= 0; n-- {
		node = nodes[n]
		commands = append(commands, node.StepGTP()...)
	}

	return commands
}

// -------------------------------------------------------------------------

func Load(filename string) (*Node, error) {

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

func PointFromSGFString(s string, size int32) (x int32, y int32, ok bool) {

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


func SGFStringFromPoint(x, y int32) string {
	return fmt.Sprintf("%c%c", ALPHA[x], ALPHA[y])
}


func HumanStringFromPoint(x, y, size int32) string {
	const letters = "ABCDEFGHJKLMNOPQRSTUVWXYZ"
	return fmt.Sprintf("%c%v", letters[x], size - y)
}


func PointFromHumanString(s string, size int32) (x int32, y int32, ok bool) {

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
