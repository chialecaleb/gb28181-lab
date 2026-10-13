// The CLI owns the same single-instance engine as either interactive host.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/chialecaleb/gb28181-lab/internal/app"
	"github.com/chialecaleb/gb28181-lab/internal/config"
	"github.com/chialecaleb/gb28181-lab/internal/storage"
	"os"
	"time"
)

func main() {
	flag.Parse()
	kind := "snapshot"
	if flag.NArg() > 0 {
		kind = flag.Arg(0)
	}
	if e := run(kind); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(kind string) error {
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
	token := s.LocalToken()
	snap, e := s.Snapshot(token)
	if e != nil {
		return e
	}
	if kind == "snapshot" {
		return json.NewEncoder(os.Stdout).Encode(snap)
	}
	if kind != "demo" {
		return fmt.Errorf("usage: gb28181-lab [snapshot|demo]")
	}
	for _, action := range []string{"register", "keepalive", "deviceinfo", "catalog", "unregister"} {
		op, e := s.Execute(token, app.Action{Kind: action, IdempotencyKey: storage.RandomID(), ExpectedRevision: snap.Config.Revision, TimeoutSeconds: 10})
		if e != nil {
			return e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		op, e = s.Wait(ctx, token, op.ID)
		cancel()
		if e != nil {
			return e
		}
		json.NewEncoder(os.Stdout).Encode(op)
		if op.State != "completed" {
			return fmt.Errorf("demo action failed: %s", action)
		}
	}
	return nil
}
