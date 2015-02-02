package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"sync"
	"time"
)

// Use flag later
var (
	listen         = ":8080"
	listenRpc      = ":8079"
	backendAddress = "127.0.0.1:8081"
)

// Statistics
var (
	requestBytes map[string]int64
	requestLock  sync.Mutex
)

// Backend is a wrapper of net.Conn
type Backend struct {
	net.Conn
	Reader *bufio.Reader
	Writer *bufio.Writer
}

// backendQueue is going to be used to pool our connections to the
// backend in order to reduce the number of connections to our backend
// as much as possible.
var backendQueue = make(chan *Backend, 10)

func init() {
	requestBytes = make(map[string]int64)
}

func main() {
	log.Println("Hello!")

	// Initialize RPC server
	rpc.Register(&RpcServer{})
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", listenRpc)
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}
	go http.Serve(l, nil)

	// Initialize notsoproxy
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("New connection could not be established: %s", err.Error())
		}
		log.Println("New connection accepted!")

		// Handle the connection in a different goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Create a buffered reader to process the incoming connection
	reader := bufio.NewReader(conn)

	// Create a buffered writer to respond to the user
	writer := bufio.NewWriter(conn)

	// Read all the requests reusing the same connection
	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("New request could not be read: %s", err)
			}
			return
		}

		// Establish connection with the backend
		be, err := getBackend()
		if err != nil {
			log.Printf("The connection with the backend could not be established: %s", err)
			return
		}

		// Create a reader to read from the backend connection
		beReader := bufio.NewReader(be)

		// Write request to the backend
		if err := req.Write(be); err != nil {
			log.Printf("The request could not be sent to the backend: %s", err)
			return
		}
		be.Writer.Flush()

		// Spin off a new goroutine that takes care of putting the backend back
		// in the queue. We are doing this asynchronously on purpose so we don't
		// make our user waits until the backend is freed.
		go queueBackend(be)

		// Read response from the backend
		resp, err := http.ReadResponse(beReader, req)
		if err != nil {
			log.Printf("The response from the backend could not be read: %s", err)
			return
		}

		// Do statistics
		bytes := updateStats(req, resp)
		resp.Header.Set("X-Bytes", strconv.FormatInt(bytes, 10))

		// Send response to the client
		resp.Close = true
		if err := resp.Write(writer); err != nil {
			log.Printf("The response from the backend could not be sent to the cliet: %s", err)
		}
		writer.Flush()

		log.Printf("%s: %d", req.URL.Path, resp.StatusCode)
	}
}

// getBackend returns a backend. It waits up to 100ms until one becomes
// available or creates a new backend.
func getBackend() (*Backend, error) {
	select {
	case be := <-backendQueue:
		return be, nil
	case <-time.After(100 * time.Millisecond):
		be, err := net.Dial("tcp", backendAddress)
		if err != nil {
			return nil, err
		}
		return &Backend{
			Conn:   be,
			Reader: bufio.NewReader(be),
			Writer: bufio.NewWriter(be),
		}, nil
	}
}

// queueBackend returns a backend to the pool. If the queue is full we wait for
// up to a second until the backend is discarded.
func queueBackend(be *Backend) {
	select {
	case backendQueue <- be:
		// Backend re-enqueued safely, move on.
	case <-time.After(1 * time.Second):
		be.Close()
	}
}

func updateStats(req *http.Request, resp *http.Response) int64 {
	requestLock.Lock()
	defer requestLock.Unlock()
	var bytes int64 = 0
	if resp.ContentLength != -1 {
		bytes = requestBytes[req.URL.Path] + resp.ContentLength
	}
	requestBytes[req.URL.Path] = bytes
	return bytes
}
