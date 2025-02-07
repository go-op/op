package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-fuego/fuego"
)

func hotReload(s *fuego.Server) {
	reloadChan := make(chan struct{}, 1)
	fuego.GetStd(s, "/reload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		select {
		case <-r.Context().Done():
			slog.Info("client disconnected, closing its events channel")
		case <-reloadChan:
			w.Write([]byte("event: reload\n"))
			w.Write([]byte("data: Connected\n\n"))
			w.(http.Flusher).Flush()
		}
	})
	reloadChan <- struct{}{}
}
