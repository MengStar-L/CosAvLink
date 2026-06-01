package main

import (
	"context"

	"cosavlink/internal/cosplay"
	"cosavlink/internal/flaresolverr"
	"cosavlink/internal/javdb"
	"cosavlink/internal/model"
)

// App is the main Wails application struct. Its exported methods are
// automatically bound to the frontend JavaScript runtime.
type App struct {
	ctx     context.Context
	cosplay *cosplay.Client
	javdb   *javdb.Client
	fs      *flaresolverr.Client
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup is called when the Wails app starts. It initializes the
// FlareSolverr client, cosplay client, and javdb client.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.fs = flaresolverr.New(flaresolverr.Options{
		URL:         "http://localhost:8191/v1",
		MaxParallel: 2,
	})
	a.cosplay = cosplay.New()
	a.javdb = javdb.New(a.fs)
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.fs != nil {
		a.fs.Close()
	}
}

// GetVideos returns the video listing for the given page number (1-indexed).
func (a *App) GetVideos(page int) ([]model.Video, error) {
	return a.cosplay.Page(a.ctx, page)
}

// GetMagnets looks up magnet links on javdb for the given code or title.
// At least one of code or title should be non-empty.
func (a *App) GetMagnets(code, title string) (model.MagnetResult, error) {
	return a.javdb.Magnets(a.ctx, code, title)
}
