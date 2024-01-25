package tests

import (
	"BTCPrice/Server"
	pricepb "BTCPrice/protofiles"
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(msg string) error {
	args := m.Called(msg)
	return args.Error(0)
}

// MockPriceService_SubscribeServer is a mock of PriceService_SubscribeServer interface
type MockPriceService_SubscribeServer struct {
	mock.Mock
}

func (m *MockPriceService_SubscribeServer) Send(resp *pricepb.SubscribeResponse) error {
	args := m.Called(resp)
	return args.Error(0)
}

func (m *MockPriceService_SubscribeServer) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}
func (m *MockPriceService_SubscribeServer) SendHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}
func (m *MockPriceService_SubscribeServer) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}
func (m *MockPriceService_SubscribeServer) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockPriceService_SubscribeServer) SetHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockPriceService_SubscribeServer) SetTrailer(md metadata.MD) {
	m.Called(md)
}

func TestSubscribe(t *testing.T) {
	mockStream := new(MockPriceService_SubscribeServer)
	req := &pricepb.SubscribeRequest{
		Currencies: []string{"USD"},
		StartTime:  "2022-01-01T00:00:00Z",
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Set up a return value for the Context method
	mockStream.On("Context").Return(ctx)

	// Set up a return value for the Send method
	mockStream.On("Send", mock.Anything).Return(nil)

	s := &Server.Server{}

	err := s.Subscribe(req, mockStream)
	assert.NoError(t, err)
}

func TestRetrieveBTCPriceRedis(t *testing.T) {
	db, redisMock := redismock.NewClientMock() // mock redis.Client
	hist := &Server.HistoricalData{RedisClient: db}
	currency := "USD"
	timedate := time.Now().Format(time.RFC3339)
	stream := new(MockPriceService_SubscribeServer)

	redisMock.ExpectZRangeByScore("times", &redis.ZRangeBy{
		Min: strconv.FormatInt(time.Now().Unix(), 10),
		Max: strconv.FormatInt(time.Now().Unix(), 10),
	}).SetVal([]string{currency + "@" + timedate})

	// Set up an expectation for the get command
	redisMock.ExpectGet(currency + "@" + timedate).SetVal("12345")

	// Set up a return value for the Send method
	stream.On("Send", mock.Anything).Return(nil)

	err := hist.RetrieveBTCPriceRedis(currency, timedate, stream)
	assert.NoError(t, err)
}

func TestNewPublisher(t *testing.T) {
	publisher, err := Server.NewPublisher("USD")
	assert.NoError(t, err)
	assert.NotNil(t, publisher)
}

func TestNotify(t *testing.T) {
	publisher, _ := Server.NewPublisher("USD")
	price := &Server.BTCPrice{Price: 123.45}
	err := publisher.Notify(price)
	assert.NoError(t, err)
}

func TestFetchAndPublishBTCPrice(t *testing.T) {
	publisher, _ := Server.NewPublisher("USD")
	stream := new(MockPriceService_SubscribeServer)
	err := publisher.FetchAndPublishBTCPrice("USD", time.Now(), stream)
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	publisher, _ := Server.NewPublisher("USD")
	err := publisher.Close()
	assert.NoError(t, err)
}

func TestgRPC(b *testing.B) {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pricepb.NewPriceServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	b.ResetTimer()
	startTime := time.Now().Format(time.RFC3339)
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ { // Each goroutine makes 10 requests
				_, err := client.Subscribe(ctx, &pricepb.SubscribeRequest{Currencies: []string{"usd"}, StartTime: startTime})
				if err != nil {
					b.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}
