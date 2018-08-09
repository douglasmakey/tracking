package storages

import (
	"github.com/go-redis/redis"
	"sync"
	"log"
)

type RedisClient struct {
	*redis.Client
}

var redisClient *RedisClient
var once sync.Once

const key = "drivers"

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

func (c *RedisClient) AddDriverLocation(lng, lat float64, id string) {
	c.GeoAdd(
		key,
		&redis.GeoLocation{Longitude: lng, Latitude: lat, Name: id},
	)
}

func (c *RedisClient) RemoveDriverLocation(id string) {
	c.ZRem(key, id)
}

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
