// Package server exposes the web UI: a listing page plus APIs for videos
// and magnet lookups.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"cosavlink/internal/cosplay"
	"cosavlink/internal/javdb"
	"cosavlink/internal/model"
)

//go:embed web/index.html
var webFS embed.FS

// Server wires the cosplay listing and javdb magnet clients to HTTP handlers.
type Server struct {
	cosplay *cosplay.Client
	javdb   *javdb.Client
	tmpl    *template.Template
}

// New parses the embedded template and returns a Server.
func New(c *cosplay.Client, j *javdb.Client) (*Server, error) {
	tmpl, err := template.ParseFS(webFS, "web/index.html")
	if err != nil {
		return nil, err
	}
	return &Server{cosplay: c, javdb: j, tmpl: tmpl}, nil
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/videos", s.handleVideos)
	mux.HandleFunc("/api/magnets", s.handleMagnets)
	return mux
}

type indexData struct {
	Videos []model.Video
	Error  string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	var data indexData
	videos, err := s.cosplay.Page(ctx, 1)
	if err != nil {
		log.Printf("cosplay page 1: %v", err)
		data.Error = "获取 cosplay 列表失败：" + err.Error()
	}
	// Limit to 16 videos for the initial render.
	if len(videos) > 16 {
		videos = videos[:16]
	}
	data.Videos = videos

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		log.Printf("render index: %v", err)
	}
}

type videosResponse struct {
	Videos []model.Video `json:"videos"`
	Page   int           `json:"page"`
	Error  string        `json:"error,omitempty"`
}

func (s *Server) handleVideos(w http.ResponseWriter, r *http.Request) {
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
		page = p
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	videos, err := s.cosplay.Page(ctx, page)
	if err != nil {
		log.Printf("cosplay page %d: %v", page, err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(videosResponse{Page: page, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(videosResponse{Videos: videos, Page: page})
}

func (s *Server) handleMagnets(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	title := r.URL.Query().Get("title")

	// Generous timeout: the first lookup may solve a Cloudflare challenge.
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	res, err := s.javdb.Magnets(ctx, code, title)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(res)
}
