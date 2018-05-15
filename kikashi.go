package main

import (
	"fmt"
	"os"

	"github.com/veandco/go-sdl2/sdl"
)

type App struct {
	Window			*sdl.Window
	Surface			*sdl.Surface
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

	self.Window, err = sdl.CreateWindow("test", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, width, height, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}

	fmt.Printf("GetSurface...\n")

	self.Surface, err = self.Window.GetSurface()
	if err != nil {
		panic(err)
	}
}

func (self *App) Shutdown() {
	fmt.Printf("Destroy...\n")
	self.Window.Destroy()
	fmt.Printf("Quit...\n")
	sdl.Quit()
}

func (self *App) Flip() {
	self.Window.UpdateSurface()
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

func (self *App) Cls(r, g, b uint32) {
	argb := 0xff000000 + (r % 256) << 16 + (g % 256) << 8 + b
	self.Surface.FillRect(&sdl.Rect{0, 0, self.Width, self.Height}, argb)
}

func main() {

	app := new(App)
	app.Init(400, 400)

	app.Cls(96, 0, 0)
	app.Flip()

	for {
		app.Poll()
	}
}
