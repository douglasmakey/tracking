package handler

import (
	"bytes"
	"encoding/json"
	"github.com/douglasmakey/tracking/storages"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerTracking(t *testing.T) {
	driverData := []byte(`{"id": "1", "lat": -33.448890, "lng": -70.669265}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8000/tracking", bytes.NewBuffer(driverData))
	if err != nil {
		t.Fatalf("could not create test request: %v", err)
	}

	rec := httptest.NewRecorder()
	tracking(rec, req)
	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("unexpected status code %s", res.Status)
	}
}

func TestHandlerSearch(t *testing.T) {
	// Add driver
	client := storages.GetRedisClient()
	client.AddDriverLocation(-70.66925, -33.448890, "1")
	client.AddDriverLocation(-70.66925, -33.448890, "2")

	// Data and request
	jsonData := []byte(`{"lat": -33.448890, "lng": -70.669265, "limit": 2}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8000/search", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("could not create test request: %v", err)
	}

	rec := httptest.NewRecorder()
	search(rec, req)
	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("unexpected status code %s", res.Status)
	}

	result := []struct {
		Name      string
		Latitude  float64
		Longitude float64
	}{}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		t.Errorf("could not decode response %v", err)
	}

	if result[0].Name != "1" {
		t.Error("the first item in result could be driver one")
	}

	// Remove drivers
	client.RemoveDriverLocation("1")
	client.RemoveDriverLocation("2")
}
