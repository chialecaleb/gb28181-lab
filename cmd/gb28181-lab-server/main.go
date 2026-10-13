package main

import (
	"context"
	"fmt"
	"github.com/chialecaleb/gb28181-lab/internal/app"
	"github.com/chialecaleb/gb28181-lab/internal/config"
	"github.com/chialecaleb/gb28181-lab/internal/transport"
	"github.com/chialecaleb/gb28181-lab/web"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Lab server failed:", e)
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
	l, e := net.Listen("tcp4", c.HTTPBind)
	if e != nil {
		return fmt.Errorf("HTTP port unavailable")
	}
	server := transport.NewServer(l.Addr().String(), transport.NewHTTP(s, l.Addr().String(), web.Assets()))
	fmt.Println("GB28181 Lab listening at http://" + l.Addr().String())
	fmt.Println("Open the workbench and enter the local access.token from LAB_DATA_DIR; token is never logged.")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(l) }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case <-done:
		return fmt.Errorf("HTTP server stopped unexpectedly")
	}
}
