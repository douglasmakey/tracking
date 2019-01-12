package handler

import (
	"github.com/douglasmakey/tracking/handler/v2"
	"net/http"
)

func NewHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/tracking", tracking)
	mux.HandleFunc("/search", search)

	// V2
	mux.HandleFunc("/v2/search", v2.SearchV2)
	mux.HandleFunc("/v2/cancel", v2.CancelRequest)
	return mux
}
