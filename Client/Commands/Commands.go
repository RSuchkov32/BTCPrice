package commands

import (
	pricepb "BTCPrice/protofiles"
	"context"
	"io"
	"log"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const (
	Retries = 5
)

var RootCmd = &cobra.Command{
	Use:   "client",
	Short: "A client for the PriceService",
}

var StartTime string

var UsdCmd = &cobra.Command{
	Use:   "usd",
	Short: "Get BTC price in USD",
	Run: func(cmd *cobra.Command, args []string) {
		runClient([]string{"USD"}, StartTime)
	},
}

var EurCmd = &cobra.Command{
	Use:   "eur",
	Short: "Get BTC price in EUR",
	Run: func(cmd *cobra.Command, args []string) {
		runClient([]string{"EUR"}, StartTime)
	},
}
var AllCmd = &cobra.Command{
	Use:   "all",
	Short: "Get BTC price in both USD and EUR",
	Run: func(cmd *cobra.Command, args []string) {
		runClient([]string{"USD", "EUR"}, StartTime)
	},
}

func runClient(currencies []string, startTime string) {
	var cc *grpc.ClientConn
	var err error

	// Retry the connection up to 5 times
	for i := 0; i < Retries; i++ {
		cc, err = grpc.Dial("localhost:50051", grpc.WithInsecure())
		if err == nil {
			break
		}

		log.Printf("Could not connect: %v, retrying...", err)
		time.Sleep(2 * time.Second)
	}

	// If the connection still failed after 5 tries, stop the program
	if err != nil {
		log.Fatalf("Could not connect: %v", err)
	}

	defer cc.Close()

	c := pricepb.NewPriceServiceClient(cc)

	// If no start time is provided, use the current time
	if startTime == "" {
		startTime = time.Now().Format(time.RFC3339)
	}

	stream, err := c.Subscribe(context.Background(), &pricepb.SubscribeRequest{Currencies: currencies, StartTime: startTime})
	if err != nil {
		log.Fatalf("Error while calling Subscribe: %v", err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// The stream has ended
			log.Fatal("The stream has ended")
		}
		if err != nil {
			log.Fatalf("Error while reading stream: %v", err)
		}

		log.Printf("Received a new price update: %v", msg)
	}
}
