package main

import (
	"log"
	"net"

	"BTCPrice/Server"

	"google.golang.org/grpc"

	pricepb "BTCPrice/protofiles"
)

func main() {
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// An instance of server
	server := &Server.Server{}

	s := grpc.NewServer()
	pricepb.RegisterPriceServiceServer(s, server)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
