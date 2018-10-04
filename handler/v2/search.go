package v2

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/douglasmakey/tracking/storages"
	"github.com/douglasmakey/tracking/tasks"
)

func SearchV2(w http.ResponseWriter, r *http.Request) {
	rClient := storages.GetRedisClient()
	// We use Redis to keep a key unique for each request.
	// With this key also we will know if the request is active or if the user canceled the request.
	requestID, err := rClient.Incr("request_id").Result()
	if err != nil {
		return
	}
	key := strconv.Itoa(int(requestID))

	// Set true value for the key and also the expiration time, this expiration time is the duration that has the request to find a driver.
	rClient.Set(key, true, time.Minute*4)
	body := struct {
		Lat, Lng float64
	}{}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("could not decode request: %v", err)
		http.Error(w, "could not decode request", http.StatusInternalServerError)
		return
	}

	// We create a new task and launch with a goroutine.
	rTask := tasks.NewRequestDriverTask(key, fmt.Sprintf("requestor_%s", key), body.Lat, body.Lng)
	go rTask.Run()

	// Return 200 and request_id
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"request_id": %s}`, key)))

}

func CancelRequest(w http.ResponseWriter, r *http.Request) {
	rClient := storages.GetRedisClient()

	body := struct {
		RequestID string `json:"request_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("could not decode request: %v", err)
		http.Error(w, "could not decode request", http.StatusInternalServerError)
		return
	}

	rClient.Set(body.RequestID, false, time.Minute*1)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return

}
