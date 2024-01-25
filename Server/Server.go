package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"

	pricepb "BTCPrice/protofiles"
)

const tickerRate = 5 * time.Second

type server struct {
	pricepb.UnimplementedPriceServiceServer
}

type APIResponse struct {
	Time struct {
		UpdatedISO string `json:"updatedISO"`
	} `json:"time"`
	Bpi map[string]struct {
		RateFloat float64 `json:"rate_float"`
	} `json:"bpi"`
}

type BTCPrice struct {
	Time  time.Time `json:"timedate"`
	Price float64   `json:"price"`
}

func fetchBTCPrice(currency string, timedate time.Time) (*BTCPrice, error) {
	//Fetch the price from the API
	resp, err := http.Get("https://api.coindesk.com/v1/bpi/currentprice.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch BTC price: %w", err)
	}
	defer resp.Body.Close()

	//Decode the response
	var data APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	//Get the price for the requested currency
	price, ok := data.Bpi[currency]
	if !ok {
		return nil, fmt.Errorf("currency not found: %s", currency)
	}

	return &BTCPrice{Price: price.RateFloat}, nil
}

func (s *server) Subscribe(req *pricepb.SubscribeRequest, stream pricepb.PriceService_SubscribeServer) error {
	currencies := req.GetCurrencies()

	//Send the price every 5 seconds
	ticker := time.NewTicker(tickerRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, currency := range currencies {
				//Fetch the price
				price, err := fetchBTCPrice(currency, time.Now())
				if err != nil {
					log.Println(err)
					return err
				}
				//Send the price to the client
				res := &pricepb.SubscribeResponse{
					Currency: currency,
					Timedate: time.Now().Format(time.RFC3339),
					Price:    price.Price,
				}
				if err := stream.Send(res); err != nil {
					return err
				}
			}
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pricepb.RegisterPriceServiceServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
