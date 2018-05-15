package main

import (
	"fmt"
	"os"

	"github.com/veandco/go-sdl2/sdl"
)

const TITLE = "Kikashi"

// -------------------------------------------------------------------------

func main() {

	app := new(App)
	app.Init(19, 24, 20)

	for {
		app.Poll()
	}
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


func (self *App) Init(SZ, cell_width, margin int32) {

	self.SZ = SZ

	self.CellWidth = cell_width
	self.Offset = self.CellWidth / 2
	self.Margin = margin

	self.PixelWidth = (self.CellWidth * SZ) + (self.Margin * 2)
	self.PixelHeight = self.PixelWidth

	self.InitSDL()

	self.DrawGrid()
	self.Flip()
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
