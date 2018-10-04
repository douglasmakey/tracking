# Tracking Service with Go and Redis.

[README V2](https://github.com/douglasmakey/tracking/blob/master/README_V2.md)

Imagine that we work at a startup like Uber and we need to create a new service that saves drivers locations every given time and processes it. This way, when someone requests a driver we can find out which drivers are closer to our picking point.

This is the core of our service. Save the locations and search nearby drivers. For this service we are using Go and Redis.

### Redis
Redis is an open source (BSD licensed), in-memory data structure store, used as a database, cache and message broker. It supports data structures such as strings, hashes, lists, sets, sorted sets with range queries, bitmaps, hyperloglogs and geospatial indexes with radius queries. [Redis](http://redis.io)

Redis has multiple functions but for the purpose of this service we are going to focus on its geospatial functions.

First we need to install Redis, I recommend using Docker running a container with Redis. By simply following this command, we will have a container running Redis in our machine.

```bash
docker run -d -p 6379:6379 redis
```


# Let's start coding

We are going to write a basic implementation for this service since I want to write other articles on how to improve this service. I will use this code as a base on my next articles.

For this service we need to use the package "github.com/go-redis/redis" that provides a Redis client for Golang.

Create a new project(folder) in your workdir. In my case I will call it 'tracking'. First we need to install the package.

```bash
go get -u github.com/go-redis/redis
```

Then we create the file 'storages/redis.go' that contains the implementation that will help us  getting a Redis client and some functions to work with geospatial.

We now create a struct that contains a pointer to the redis client. This pointer will have the functions that help us with this service, we also create a constant with the key name for our set in redis.

```go
type RedisClient struct { *redis.Client }
const key = "drivers"

```

For the function to get the Redis client, we are going to use the singleton pattern with the help of the sync package and its Once.Do functionality.

In software engineering, the singleton pattern is a software design pattern that restricts the instantiation of a class to one object. This is useful when exactly one object is needed to coordinate actions across the system. If you want to read more about [Singleton Pattern](https://en.wikipedia.org/wiki/Singleton_pattern).

But how works Once.Do, the struct `sync.Once` has an atomic counter and it uses `atomic.StoreUint32` to set a value to 1, when the function has been called, and then `atomic.LoadUint32` to see if it needs to be called again. For this basic implementation GetRedisClient will be called from two endpoints but we only want to get one instance.

```go
var once sync.Once
var redisClient *RedisClient

func GetRedisClient() *RedisClient {
	once.Do(func() {
		client := redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		})

		redisClient = &RedisClient{client}
	})

	_, err := redisClient.Ping().Result()
	if err != nil {
		log.Fatalf("Could not connect to redis %v", err)
	}

	return redisClient
}

```

Then we create three functions for the RedisClient.

AddDriverLocation: Add the specified geospatial item (latitude, longitude, name "in this case name is the driver id") to the specified key, do you remember the key that we defined at the beginning for our Set in Redis ? This is it.

```go
func (c *RedisClient) AddDriverLocation(lng, lat float64, id string) {
	c.GeoAdd(
		key,
		&redis.GeoLocation{Longitude: lng, Latitude: lat, Name: id},
	)
}

```

RemoveDriverLocation: The client redis does not have the function GeoDel because GEODEL command does not exist, so we can use ZREM in order to remove elements. The Geo index structure is just a sorted set.

```go
func (c *RedisClient) RemoveDriverLocation(id string) {
	c.ZRem(key, id)
}

```

SearchDrivers: the function GeoRadius implements the command GEORADIUS that returns the members of a sorted set populated with geospatial information using GEOADD, which are within the borders of the area specified with the center location and the maximum distance from the center (the radius). If you want to learn more about this go [GEORADIUS](https://redis.io/commands/georadius)

```go
func (c *RedisClient) SearchDrivers(limit int, lat, lng, r float64) []redis.GeoLocation {
	/*
	WITHDIST: Also return the distance of the returned items from the
	specified center. The distance is returned in the same unit as the unit
	specified as the radius argument of the command.
	WITHCOORD: Also return the longitude,latitude coordinates of the matching items.
	WITHHASH: Also return the raw geohash-encoded sorted set score of the item,
	in the form of a 52 bit unsigned integer. This is only useful for low level
	hacks or debugging and is otherwise of little interest for the general user.
	 */
	 
	res, _ := c.GeoRadius(key, lng, lat, &redis.GeoRadiusQuery{
		Radius:      r,
		Unit:        "km",
		WithGeoHash: true,
		WithCoord:   true,
		WithDist:    true,
		Count:       limit,
		Sort:        "ASC",
	}).Result()

	return res
}
```

Next, create a main.go

```golang
package main

import (
	"net/http"
	"fmt"
	"log"
)

func main() {
	// We create a simple httpserver
	server := http.Server{
		Addr:    fmt.Sprint(":8000"),
		Handler: NewHandler(),
	}

	// Run server
	log.Printf("Starting HTTP Server. Listening at %q", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Printf("%v", err)
	} else {
		log.Println("Server closed ! ")
	}

}
```
We create a simple server using http.Server.

Then we create file 'handler/handler.go' that contains the endpoints for our application.

```golang
func NewHandler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("tracking", tracking)
	mux.HandleFunc("search", search)
	return mux
}

```

We use http.ServeMux to handle our endpoints, we create two endpoints for our service.

The first endpoint 'tracking' let's us save the last location sent from a driver, in this case we only want to save the last location. We could modify this endpoint so that previous locations are saved in another database.

```go
func tracking(w http.ResponseWriter, r *http.Request) {
	// crate an anonymous struct for driver data.
	var driver = struct {
		ID string `json:"id"`
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
```

The second endpoint is 'search' with this endpoint we can find all drivers near a given point,

```go
// search receives lat and lng of the picking point and searches drivers about this point.
func search(w http.ResponseWriter, r *http.Request) {
	rClient := storages.GetRedisClient()

	body := struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
		Limit int `json:"limit"`
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

```

# Let's test the service

First, run the server.
```bash
go run main.go
```

Next, we need to add four drivers locations.

![Map example screenshot](https://github.com/douglasmakey/tracking/blob/master/example.png?raw=true)

We add four drivers as above in the map, the lines green show distance between picking point and drivers.


```bash
curl -i --header "Content-Type: application/json" --data '{"id": "1", "lat": -33.44091, "lng": -70.6301}' http://localhost:8000/tracking

curl -i --header "Content-Type: application/json" --data '{"id": "2", "lat": -33.44005, "lng": -70.63279}' http://localhost:8000/tracking

curl -i --header "Content-Type: application/json" --data '{"id": "3", "lat": -33.44338, "lng": -70.63335}' http://localhost:8000/tracking

curl -i --header "Content-Type: application/json" --data '{"id": "4", "lat": -33.44186, "lng": -70.62653}' http://localhost:8000/tracking
```

Since we now have the locations of the drivers, we can do a spacial search.


we will look for 4 nearby drivers

```bash
curl -i --header "Content-Type: application/json" --data '{"lat": -33.44262, "lng": -70.63054, "limit": 5}' http://localhost:8000/search
```

As you will see the result matches with the map, see the lines greens in the map.

```bash
HTTP/1.1 200 OK
Content-Type: application/json
Date: Wed, 08 Aug 2018 05:07:57 GMT
Content-Length: 456

[
    {
        "Name": "1",
        "Longitude": -70.63009768724442,
        "Latitude": -33.44090957099124,
        "Dist": 0.1946,
        "GeoHash": 861185092131738
    },
    {
        "Name": "3",
        "Longitude": -70.63334852457047,
        "Latitude": -33.44338092412159,
        "Dist": 0.2741,
        "GeoHash": 861185074815667
    },
    {
        "Name": "2",
        "Longitude": -70.63279062509537,
        "Latitude": -33.44005030051822,
        "Dist": 0.354,
        "GeoHash": 861185086448695
    },
    {
        "Name": "4",
        "Longitude": -70.62653034925461,
        "Latitude": -33.44186009142599,
        "Dist": 0.3816,
        "GeoHash": 861185081504625
    }
]
```


Look up for the nearest driver

```bash
curl -i --header "Content-Type: application/json" --data '{"lat": -33.44262, "lng": -70.63054, "limit": 1}' http://localhost:8000/search
```

Result

```bash
HTTP/1.1 200 OK
Content-Type: application/json
Date: Wed, 08 Aug 2018 05:12:24 GMT
Content-Length: 115

[{"Name":"1","Longitude":-70.63009768724442,"Latitude":-33.44090957099124,"Dist":0.1946,"GeoHash":861185092131738}]
```
