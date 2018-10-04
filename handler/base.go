package handler

import (
	"net/http"
	"github.com/douglasmakey/tracking/handler/v2"
)

func NewHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/tracking", tracking)
	mux.HandleFunc("/search", search)

	// V2
	mux.HandleFunc("/v2/search", v2.SearchV2)
	mux.HandleFunc("/v2/cancel", v2.CancelRequest)
	return mux
}