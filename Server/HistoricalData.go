package Server

import (
	pricepb "BTCPrice/protofiles"
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

const separator = "@"
const redisKey = "times"

type HistoricalData struct {
	RedisClient *redis.Client
}

// Create a new historical data instance
func NewHistoricalData() (*HistoricalData, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379" // default value
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "" // default value
	}

	redisDB := os.Getenv("REDIS_DB")
	db, err := strconv.Atoi(redisDB)
	if err != nil || db < 0 {
		db = 0 // default value
	}

	redis := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       db,
	})

	return &HistoricalData{RedisClient: redis}, nil
}

// Retrieve the BTC price from the cache
func (hist *HistoricalData) RetrieveBTCPriceRedis(currency string, timedate string, stream pricepb.PriceService_SubscribeServer) error {
	ctx := context.Background()
	keys, err := hist.RetrievePriceFromRedis(ctx, currency, timedate)
	if err != nil {
		return err
	}
	return hist.SendPricesToClient(ctx, currency, keys, stream)
}

// Retrieve the BTC price from the cache
func (hist *HistoricalData) RetrievePriceFromRedis(ctx context.Context, currency string, timedate string) ([]string, error) {
	//priceKey := currency + "@" + timedate
	// Parse the timedate from the request
	reqTimedate, err := time.Parse(time.RFC3339, timedate)
	if err != nil {
		log.Printf("Invalid timedate: %v", err)
		return nil, err
	}

	priceKeys, err := hist.RedisClient.ZRangeByScore(ctx, "times", &redis.ZRangeBy{
		Min: strconv.FormatInt(reqTimedate.Unix(), 10),
		Max: strconv.FormatInt(time.Now().Unix(), 10),
	}).Result()
	if err != nil {
		log.Printf("Error fetching times: %v", err)
		return nil, err
	}

	// Filter the keys to only include those for the requested currency
	var filteredKeys []string
	for _, key := range priceKeys {
		if strings.HasPrefix(key, currency+separator) {
			filteredKeys = append(filteredKeys, key)
		}
	}

	return filteredKeys, nil
}

// Send the prices to the client
func (hist *HistoricalData) SendPricesToClient(ctx context.Context, currency string, keys []string, stream pricepb.PriceService_SubscribeServer) error {
	// For each key, retrieve the price from the cache
	for _, key := range keys {
		priceStr, err := hist.RedisClient.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Error fetching price for %s: %v", key, err)
			continue
		}

		// Convert priceStr to float64
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			log.Printf("Error converting price to float64: %v", err)
			continue
		}

		res := &pricepb.SubscribeResponse{
			Currency: currency,
			Timedate: strings.Split(key, separator)[1],
			Price:    priceFloat,
		}
		if err := stream.Send(res); err != nil {
			log.Printf("Error sending response: %v", err)
			return err
		}
	}

	return nil
}

// Cache the price in Redis
func (hist *HistoricalData) CachePrice(ctx context.Context, currency string, timedate time.Time, price float64) error {
	// Store the price and timedate in the cache for future use
	timedateStr := timedate.Format(time.RFC3339)
	priceKey := currency + separator + timedateStr
	hist.RedisClient.Set(ctx, priceKey, price, 0)
	hist.RedisClient.ZAdd(ctx, redisKey, &redis.Z{Score: float64(timedate.Unix()), Member: priceKey})

	return nil
}

func (hist *HistoricalData) Close() error {
	if hist.RedisClient != nil {
		return hist.RedisClient.Close()
	}
	return nil
}
