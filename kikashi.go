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
	app.Init(400, 400)

	app.Cls(96, 0, 0)
	app.Flip()

	for {
		app.Poll()
	}
}

// -------------------------------------------------------------------------

type App struct {
	Window			*sdl.Window
	Renderer		*sdl.Renderer
	Width			int32
	Height			int32
}


func (self *App) Init(width, height int32) {

	self.Width = width
	self.Height = height

	fmt.Printf("Init...\n")

	err := sdl.Init(sdl.INIT_EVERYTHING)
	if err != nil {
		panic(err)
	}

	fmt.Printf("CreateWindow...\n")

	self.Window, err = sdl.CreateWindow(TITLE, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, width, height, sdl.WINDOW_SHOWN)
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
	self.Renderer.FillRect(&sdl.Rect{0, 0, self.Width, self.Height})
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
