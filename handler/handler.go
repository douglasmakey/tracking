package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/douglasmakey/tracking/storages"
)

// tracking receive the driver coord and saves the coord in redis
func tracking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// crate an anonymous struct for driver data.
	var driver = struct {
		ID  string  `json:"id"`
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	}{}

	rClient := storages.GetRedisClient()

	if err := json.NewDecoder(r.Body).Decode(&driver); err != nil {
		log.Printf("could not decode request: %v", err)
		http.Error(w, "could not decode request", http.StatusInternalServerError)
		return
	}

	// Add new location
	// You can save locations in another db
	rClient.AddDriverLocation(driver.Lng, driver.Lat, driver.ID)

	w.WriteHeader(http.StatusOK)
	return
}

// search receives lat and lng of the picking point and searches drivers about this point.
func search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rClient := storages.GetRedisClient()

	body := struct {
		Lat   float64 `json:"lat"`
		Lng   float64 `json:"lng"`
		Limit int     `json:"limit"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("could not decode request: %v", err)
		http.Error(w, "could not decode request", http.StatusInternalServerError)
		return
	}

	drivers := rClient.SearchDrivers(body.Limit, body.Lat, body.Lng, 15)
	data, err := json.Marshal(drivers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
	return
}
