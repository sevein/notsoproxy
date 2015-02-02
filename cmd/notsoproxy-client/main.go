package main

import (
	"log"
	"net/rpc"
	"strconv"
)

var (
	rpcAddress = "127.0.0.1:8079"
)

type Stats struct {
	RequestBytes map[string]int64
}

type Empty struct{}

func main() {
	client, err := rpc.DialHTTP("tcp", rpcAddress)
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}

	var reply Stats
	err = client.Call("RpcServer.GetStats", &Empty{}, &reply)
	if err != nil {
		log.Fatalf("Failed to GetStats: %s", err)
	}

	for k, v := range reply.RequestBytes {
		log.Printf("%s (%s bytes)", k, strconv.FormatInt(v, 10))
	}
}
