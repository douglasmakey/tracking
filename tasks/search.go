package tasks

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/douglasmakey/tracking/storages"
)

// These are the reasons which a request is invalid.
var (
	ErrExpired  = error.New("request expired")
	ErrCanceled = errors.New("request canceled")
)

// RequestDriverTask is a simple struct that contains info about the user, request and driver, you can add more information if you want.
type RequestDriverTask struct {
	ID       string
	UserID   string
	Lat, Lng float64
	DriverID string
}

// NewRequestDriverTask create and return a pointer to RequestDriverTask
func NewRequestDriverTask(id, userID string, lat, lng float64) *RequestDriverTask {
	return &RequestDriverTask{
		ID:     id,
		UserID: userID,
		Lat:    lat,
		Lng:    lng,
	}
}

// Run is the function for executing the task, this task validating the request and launches another goroutine called 'doSearch' which does the search.
func (r *RequestDriverTask) Run() {
	// We create a new ticker with 30s time duration, this it means that each 30s the task executes the search for a driver.
	ticker := time.NewTicker(time.Second * 30)

	// With the done channel, we receive if the driver was found
	done := make(chan bool, 1)

	for {
		// The select statement lets a goroutine wait on multiple communication operations.
		select {
		case <-ticker.C:
			switch r.validateRequest() {
			case nil:
				log.Println(fmt.Sprintf("Search Driver - Request %s for Lat: %f and Lng: %f", r.ID, r.Lat, r.Lng))
				go r.doSearch(done)
			case ErrExpired:
				// Notify to user that the request expired.
				sendInfo(r, "Sorry, we did not find any driver.")
				return
			case ErrCanceled:
				log.Printf("Request %s has been canceled. ", r.ID)
				return
			default: // defensive programming: expected the unexpected
				log.Printf("unexpected error: %v", err)
				return
			}

		case <-done:
			sendInfo(r, fmt.Sprintf("Driver %s found", r.DriverID))
			ticker.Stop()
			return
		}
	}
}

// validateRequest validates if the request is valid and return a string like a reason in case not.
func (r *RequestDriverTask) validateRequest() error {
	rClient := storages.GetRedisClient()
	keyValue, err := rClient.Get(r.ID).Result()
	if err != nil {
		// Request has been expired.
		return ErrExpired
	}

	isActive, _ := strconv.ParseBool(keyValue)
	if !isActive {
		// Request has been canceled.
		return ErrCanceled
	}

	return nil
}

// doSearch do search of driver and send signal to the channel.
func (r *RequestDriverTask) doSearch(done chan bool) {
	rClient := storages.GetRedisClient()
	drivers := rClient.SearchDrivers(1, r.Lat, r.Lng, 5)
	if len(drivers) == 1 {
		// Driver found
		// Remove driver location, we can send a message to the driver for that it does not send again its location to this service.
		rClient.RemoveDriverLocation(drivers[0].Name)
		r.DriverID = drivers[0].Name
		done <- true
	}

	return
}

// sendInfo this func is only example, you can use another services, websocket or push notification for send data to user.
func sendInfo(r *RequestDriverTask, message string) {
	log.Println("Message to user:", r.UserID)
	log.Println(message)
}
