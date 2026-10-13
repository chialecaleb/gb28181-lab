//go:build windows

package main

import (
	"context"
	"fmt"
	"github.com/chialecaleb/gb28181-lab/internal/app"
	"github.com/chialecaleb/gb28181-lab/internal/config"
	"github.com/chialecaleb/gb28181-lab/internal/transport"
	"github.com/chialecaleb/gb28181-lab/web"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"os"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Lab desktop failed:", e)
		os.Exit(1)
	}
}
func run() error {
	c, e := config.Load(".", os.LookupEnv)
	if e != nil {
		return e
	}
	external, e := app.LoadExternal(c.DataDir)
	if e != nil {
		return e
	}
	s, e := app.Open(app.Options{Runtime: c, External: external})
	if e != nil {
		return e
	}
	defer s.Close()
	return wails.Run(&options.App{Title: "GB28181 Lab · M0 信令工作台", Width: 1440, Height: 920, MinWidth: 1000, MinHeight: 700, AssetServer: &assetserver.Options{Assets: web.Assets()}, Bind: []interface{}{transport.NewDesktop(s)}, OnShutdown: func(context.Context) { s.Close() }})
}
