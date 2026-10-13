// Package transport contains thin adapters over the shared application service.
package transport

import (
	"github.com/chialecaleb/gb28181-lab/internal/app"
	"github.com/chialecaleb/gb28181-lab/internal/evidence"
)

// Desktop is the complete Wails binding surface. Never bind app.Service: its
// trusted host methods intentionally include local secrets and raw evidence.
type Desktop struct{ service *app.Service }

func NewDesktop(s *app.Service) *Desktop           { return &Desktop{service: s} }
func (d *Desktop) Snapshot() (app.Snapshot, error) { return d.service.Snapshot(d.service.LocalToken()) }
func (d *Desktop) Execute(a app.Action) (app.Operation, error) {
	return d.service.Execute(d.service.LocalToken(), a)
}
func (d *Desktop) SaveConfig(c app.Config) (app.Config, error) {
	return d.service.SaveConfig(d.service.LocalToken(), c)
}
func (d *Desktop) Cancel(id string) (app.Operation, error) {
	return d.service.Cancel(d.service.LocalToken(), id)
}
func (d *Desktop) Traces() (evidence.Snapshot, error) {
	return d.service.Traces(d.service.LocalToken(), 0, 1000)
}
func (d *Desktop) Evidence() (map[string]any, error) {
	return d.service.Evidence(d.service.LocalToken())
}
