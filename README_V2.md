# Tracking Service V2

Do you remember my last article where I wrote a service to look for a driver like uber? If not, you can check [here](https://github.com/douglasmakey/tracking/blob/master/README.md) So now, we going to write the V2 of our service.

The current state of our service, when a user consumes the resource 'search', the user receives a response with the closer driver to him. But what would happen if there are no drivers close to the user? We don't want the service client doing a big amount of requests to the same endpoint to look for a driver. What we want to do is to follow the pattern Uber uses and that is that our client makes only one request and this request raises a task that looks for a driver to us for X time and, later on, the user receives the result.


For doing this we are going to use some of the tools Go provides us: Goroutines, Channels and time.Ticker struct.

NOTE: This will be a basic implementation.

## Goroutine
~~A goroutine is a lightweight, costing little more than the allocation of stack space. And the stacks start small, so they are cheap, and grow by allocating (and freeing) heap storage as required.~~

To run a function as a goroutine, just simply put the keyword `go` before a func call. When the func is done, the goroutine exits, silently.


```go
go list.Sort()  // run list.Sort concurrently; don't wait for it.
```

NOTE: Goroutines run in the same address space, so access to shared memory must be synchronized. The sync package provides useful primitives, although you won't need them much in Go as there are other primitives.

[Effective Go - Goroutines ](https://golang.org/doc/effective_go.html?#goroutines)

## Channels
~~Channels are a typed conduit through which you can send and receive values with the channel operator, <-.~~

```go
ch <- v    // Send v to channel ch.
v := <-ch  // Receive from ch, and
           // assign value to v.
```
(The data flows in the direction of the arrow.)

Like maps, channels are allocated with make, and the resulting value acts as a reference to an underlying data structure. If an optional integer parameter is provided, it sets the buffer size for the channel. The default is zero, for an unbuffered or synchronous channel.

```go
ci := make(chan int)            // unbuffered channel of integers
cj := make(chan int, 0)         // unbuffered channel of integers
cs := make(chan *os.File, 100)  // buffered channel of pointers to Files
```
Unbuffered channels combine communication—the exchange of a value—with synchronization—guaranteeing that two calculations (goroutines) are in a known state.


There are lots of nice idioms using channels. Here's one to get us started. In the previous section we launched a sort in the background. A channel can allow the launching goroutine to wait for the sort to complete.

```go
c := make(chan int)  // Allocate a channel.
// Start the sort in a goroutine; when it completes, signal on the channel.
go func() {
    list.Sort()
    c <- 1  // Send a signal; value does not matter.
}()
doSomethingForAWhile()
<-c   // Wait for sort to finish; discard sent value.
```

Receivers always block until there is data to receive. If the channel is unbuffered, the sender blocks until the receiver has received the value. If the channel has a buffer, the sender blocks only until the value has been copied to the buffer; if the buffer is full, this means waiting until some receiver has retrieved a value.


[Effective Go - Channels ](https://golang.org/doc/effective_go.html?#channels)

## time.Ticker

Timers are for when you want to do something once in the future - tickers are for when you want to do something repeatedly at regular intervals. Here’s an example of a ticker that ticks periodically until we stop it.

```go
package main

import "time"
import "fmt"

func main() {

    // Tickers use a similar mechanism to timers: a
    // channel that is sent values. Here we'll use the
    // `range` builtin on the channel to iterate over
    // the values as they arrive every 500ms.
    ticker := time.NewTicker(500 * time.Millisecond)
    go func() {
        for t := range ticker.C {
            fmt.Println("Tick at", t)
        }
    }()

    // Tickers can be stopped like timers. Once a ticker
    // is stopped it won't receive any more values on its
    // channel. We'll stop ours after 1600ms.
    time.Sleep(1600 * time.Millisecond)
    ticker.Stop()
    fmt.Println("Ticker stopped")
}

```

# Let's start coding

First, we will create a new folder called tasks, inside we create a 'request.go' that contains the code to do the search.

```go
// FILE: tasks/search.go

// These are the reasons which a request is invalid.
var (
	ErrExpired  = errors.New("request expired")
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
		Lat:lat,
		Lng:lng,
	}
}
```

We implement the **Run** method, this func will be launched from the handler.

```go
// FILE: tasks/search.go

// Run is the function for executing the task, this task validating the request and launches another goroutine called 'doSearch' which does the search.
func (r *RequestDriverTask) Run() {
	// We create a new ticker with 30s time duration, this it means that each 30s the task executes the search for a driver.
	ticker := time.NewTicker(time.Second * 30)

	// With the done channel, we receive if the driver was found
	done := make(chan struct{})

	for {
		// The select statement lets a goroutine wait on multiple communication operations.
		select {
		case <-ticker.C:
			err := r.validateRequest()
			switch err {
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

		case _, ok := <-done:
			if !ok {
				sendInfo(r, fmt.Sprintf("Driver %s found", r.DriverID))
				ticker.Stop()
				return
			}
		}
	}
}

```

Ok, now we going to create two methods for RequestDriverTask.

The first method is **validateRequest**, this function validates the key, if the key is active or if the key expired and will return error like a reason if the request is not valid.

The second method is **doSearch**, this function uses our RedisClient and its function SearchDrivers for doing search.

```go
// FILE: tasks/search.go

// validateRequest validates if the request is valid and return an error like a reason in case not.
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

// doSearch do search of driver and close to the channel.
func (r *RequestDriverTask) doSearch(done chan struct{}) {
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
```

the function **sendInfo** is just example, you can implement another service or push notification or WebSocket if you want, I want to write another article where I implement an example using FCM with a little library that I wrote [go-fcm](https://github.com/douglasmakey/go-fcm) to notify the user.

```go
// sendInfo this func is only example, you can use another services, websocket or push notification for send data to user.
func sendInfo(r *RequestDriverTask, message string) {
	log.Println("Message to user:", r.UserID)
	log.Println(message)
}
```

Ok, we already have the functions for the search task, now we need to create the new endpoints for  our service, we going to create a new folder into handler called 'v2' and inside we create 'search.go'

```go
// FILE: handler/v2/search.go

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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"request_id": %s}`, key)))

}
```

Next, we create the handler for endpoint 'v2/cancel' to cancel the request, because if the user doesn't want to wait for the search it can cancel the request.

```go
// FILE: handler/v2/search.go

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
	w.WriteHeader(http.StatusOK)
	return

}
```

Finally, we need to add new endpoints to our routes.

```go
// FILE: handler/base.go

import (
	"net/http"
	"github.com/douglasmakey/tracking/handler/v2"
)

func NewHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/tracking", tracking)
	mux.HandleFunc("/search", search)

	//V2
	mux.HandleFunc("/v2/search", v2.SearchV2)
	mux.HandleFunc("/v2/cancel", v2.CancelRequest)
	return mux
}
```

# Example

Look up for the nearest driver

```bash
curl -i --header "Content-Type: application/json" --data '{"lat": -33.44262, "lng": -70.63054}' http://localhost:8000/v2/search


HTTP/1.1 200 OK
Date: Sat, 29 Sep 2018 15:33:48 GMT
Content-Length: 17
Content-Type: application/json

{"request_id": 1}
```

But if drivers are not close client location and pass 4 minutes or the time duration that we set, the request will be expired without finding any drivers.

Server Log

```bash
2018/09/30 01:57:53 Starting HTTP Server. Listening at ":8000"
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Search Driver - Request 1 for Lat: -33.442620 and Lng: -70.630540
Message to user:  requestor_1
Sorry, we did not find any driver.
```

We going to do another request to 'v2/search', but after 1 minute in another terminal, we send driver location to service.

```bash
// Another Terminal
curl -i --header "Content-Type: application/json" --data '{"id": "1", "lat": -33.44091, "lng": -70.6301}' http://localhost:8000/tracking

```


```bash
// Terminal with main
2018/09/30 02:12:03 Starting HTTP Server. Listening at ":8000"
2018/09/30 02:12:38 Search Driver - Request 2 for Lat: -33.442620 and Lng: -70.630540
2018/09/30 02:13:08 Search Driver - Request 2 for Lat: -33.442620 and Lng: -70.630540
2018/09/30 02:13:38 Search Driver - Request 2 for Lat: -33.442620 and Lng: -70.630540
2018/09/30 02:13:38 Message to user: requestor_2
2018/09/30 02:13:38 Driver 1 found
```

Ok, now we going to do another request to 'v2/search', but this time we going to do a request to 'v2/cancel' to cancel the order because we can not wait.

```bash
// Another Terminal
curl -i --header "Content-Type: application/json" --data '{"request_id": "3"}' http://localhost:8000/v2/cancel
```

```bash
// Terminal with main
2018/09/30 02:19:24 Starting HTTP Server. Listening at ":8000"
2018/09/30 02:19:56 Search Driver - Request 3 for Lat: -33.442620 and Lng: -70.630540
2018/09/30 02:19:56 Request 3 has been canceled. 
```