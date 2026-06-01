// Command server runs CosAvLink: a web UI that lists the latest videos from
// cosplay.jav.pw and fetches javdb.com magnet links on demand.
//
// javdb lookups go through FlareSolverr (a separate service) to solve
// Cloudflare challenges automatically.
//
// Configuration via environment variables:
//
//	PORT              HTTP port (default 8080)
//	FLARESOLVERR_URL  FlareSolverr API endpoint (default http://localhost:8191/v1)
//	MAX_PARALLEL      max concurrent FlareSolverr requests (default 2)
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cosavlink/internal/cosplay"
	"cosavlink/internal/flaresolverr"
	"cosavlink/internal/javdb"
	"cosavlink/internal/server"
)

func main() {
	addr := ":" + getenv("PORT", "8080")
	flareURL := getenv("FLARESOLVERR_URL", "http://localhost:8191/v1")
	maxParallel := getenvInt("MAX_PARALLEL", 2)

	fs := flaresolverr.New(flaresolverr.Options{
		URL:         flareURL,
		MaxParallel: maxParallel,
	})
	defer fs.Close()

	srv, err := server.New(cosplay.New(), javdb.New(fs))
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      120 * time.Second, // a first magnet lookup can be slow
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("CosAvLink 已启动: http://localhost%s  (flaresolverr=%s)", addr, flareURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("正在关闭…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
