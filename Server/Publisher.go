package Server

import (
	pricepb "BTCPrice/protofiles"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/streadway/amqp"
)

type Publisher struct {
	Channel        *amqp.Channel
	Queue          amqp.Queue
	HistoricalData *HistoricalData
}

// Create a new publisher
func NewPublisher(currency string) (*Publisher, error) {
	rabbitmqURL := os.Getenv("RABBITMQ_URL")
	if rabbitmqURL == "" {
		rabbitmqURL = "amqp://guest:guest@rabbitmq:5672/" // default value
	}

	HistoricalData, err := NewHistoricalData()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}
	conn, err := amqp.Dial(rabbitmqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}

	q, err := ch.QueueDeclare(
		"btcprice"+currency, // name
		false,               // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %v", err)
	}

	return &Publisher{Channel: ch, Queue: q, HistoricalData: HistoricalData}, nil
}

// Publish a message to the queue
func (p *Publisher) Notify(price *BTCPrice) error {
	body, err := json.Marshal(price)
	if err != nil {
		return fmt.Errorf("failed to marshal price: %v", err)
	}

	err = p.Channel.Publish(
		"",           // exchange
		p.Queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return fmt.Errorf("failed to publish a message: %v", err)
	}

	return nil
}

// Fetch the BTC price from the API
func (p *Publisher) FetchBTCPrice(currency string) (*APIResponse, error) {
	// Fetch BTC price
	resp, err := http.Get("https://api.coindesk.com/v1/bpi/currentprice.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch BTC price: %w", err)
	}
	defer resp.Body.Close()

	var data APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &data, nil
}

// Publish the price to RabbitMQ
func (p *Publisher) PublishPrice(price *BTCPrice) error {
	// Publish the price to RabbitMQ
	err := p.Notify(price)
	if err != nil {
		return fmt.Errorf("failed to notify: %w", err)
	}

	return nil
}

// Fetch the price from the API and publish it to RabbitMQ
func (p *Publisher) FetchAndPublishBTCPrice(currency string, timedate time.Time, stream pricepb.PriceService_SubscribeServer) error {
	ctx := context.Background()
	if timedate.Before(time.Now()) {
		err := p.HistoricalData.RetrieveBTCPriceRedis(currency, timedate.Format(time.RFC3339), stream)
		if err != nil {
			return err
		}
	}

	data, err := p.FetchBTCPrice(currency)
	if err != nil {
		return err
	}

	price, ok := data.Bpi[currency]
	if !ok {
		return fmt.Errorf("currency not found: %s", currency)
	}

	p.HistoricalData.CachePrice(ctx, currency, timedate, price.RateFloat)
	p.PublishPrice(&BTCPrice{Price: price.RateFloat})

	return nil
}

// Close the publisher
func (p *Publisher) Close() error {
	if p.HistoricalData != nil {
		if err := p.HistoricalData.Close(); err != nil {
			return err
		}
	}
	if p.Channel != nil {
		return p.Channel.Close()
	}
	return nil
}
