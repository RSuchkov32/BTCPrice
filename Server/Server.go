package Server

import (
	pricepb "BTCPrice/protofiles"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/streadway/amqp"
)

const tickRate = 5 * time.Second

type Server struct {
	pricepb.UnimplementedPriceServiceServer
}

// Create a new publisher
func (serv *Server) createPublisher(currency string) (*Publisher, error) {
	publisher, err := NewPublisher(currency)
	if err != nil {
		log.Printf("Failed to create a publisher for currency %s: %v", currency, err)
		return nil, err
	}
	return publisher, nil
}

// Fetch and publish the BTC price
func (serv *Server) fetchAndPublishBTCPrice(publisher *Publisher, currency string, stream pricepb.PriceService_SubscribeServer, ctx context.Context, cancel context.CancelFunc) {
	tick := time.NewTicker(tickRate)
	go func() {
		for {
			select {
			case <-tick.C:
				err := publisher.FetchAndPublishBTCPrice(currency, time.Now(), stream)
				if err != nil {
					log.Printf("error fetching BTC price: %v", err)
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Consume messages from the queue
func (serv *Server) consumeMessages(publisher *Publisher, currency string, stream pricepb.PriceService_SubscribeServer, ctx context.Context, cancel context.CancelFunc) {
	msgs, err := publisher.Channel.Consume(
		publisher.Queue.Name, // queue
		"",                   // consumer
		true,                 // auto-ack
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	if err != nil {
		log.Printf("Failed to register a consumer: %v", err)
		return
	}

	go func() {
		for d := range msgs {
			select {
			case <-ctx.Done():
				return
			default:
				serv.unmarshalAndSendResponse(d, currency, stream, cancel)
			}
		}
	}()
}

// Unmarshal the message and send it to the client
func (serv *Server) unmarshalAndSendResponse(d amqp.Delivery, currency string, stream pricepb.PriceService_SubscribeServer, cancel context.CancelFunc) {
	price := &BTCPrice{}
	if err := json.Unmarshal(d.Body, price); err != nil {
		log.Printf("Error unmarshalling price: %v", err)
		return
	}

	res := &pricepb.SubscribeResponse{
		Currency: currency,
		Timedate: time.Now().Format(time.RFC3339),
		Price:    price.Price,
	}

	if err := stream.Send(res); err != nil {
		log.Printf("Error sending response: %v", err)
		cancel()
		return
	}
}

// Subscribe to the price updates
func (serv *Server) Subscribe(req *pricepb.SubscribeRequest, stream pricepb.PriceService_SubscribeServer) error {
	currencies := req.GetCurrencies()
	timedate := req.GetStartTime()

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	for _, currency := range currencies {
		publisher, err := serv.createPublisher(currency)
		if err != nil {
			log.Fatalf("failed to create a publisher for currency %s: %v", currency, err)
		}
		defer publisher.Close()
		publisher.HistoricalData.RetrieveBTCPriceRedis(currency, timedate, stream)

		serv.fetchAndPublishBTCPrice(publisher, currency, stream, ctx, cancel)
		serv.consumeMessages(publisher, currency, stream, ctx, cancel)
	}

	<-ctx.Done()

	return nil
}
