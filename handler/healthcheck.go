package handler

import (
	"github.com/douglasmakey/tracking/storages"
	"log"
	"net/http"
)

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Get instance redis client
	redis := storages.GetRedisClient()
	// Checks that the communication with redis is alive.
	if err := redis.Ping().Err(); err != nil {
		// Put yours logs HERE
		log.Printf("redis unaccessible error: %v ", err)
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	return
}
