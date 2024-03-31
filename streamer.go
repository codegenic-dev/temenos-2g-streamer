package main

import (
	"bytes"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	PORT       = "8080"
	BUFFERSIZE = 4096
	DELAY      = 150
)

func main() {
	filenames := []string{
		"media/dani.mp3",
		"media/ethismos.mp3",
	}

	connPool := NewConnectionPool()
	go stream(connPool, filenames)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "audio/mpeg")
		w.Header().Add("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Println("Could not create flusher")
		}

		connection := &Connection{bufferChannel: make(chan []byte), buffer: make([]byte, BUFFERSIZE)}
		connPool.AddConnection(connection)
		log.Printf("%s has connected to the audio stream\n", r.Host)
		for {
			buf := <-connection.bufferChannel
			if _, err := w.Write(buf); err != nil {
				connPool.DeleteConnection(connection)
				log.Printf("%s's connection to the audio stream has been closed\n", r.Host)
				return

			}
			flusher.Flush()
			clear(connection.buffer)
		}
	})

	log.Println("Listening on port: " + PORT)
	log.Fatal(http.ListenAndServe(":"+PORT, nil))
}

type Connection struct {
	bufferChannel chan []byte
	buffer        []byte
}

type ConnectionPool struct {
	ConnectionMap map[*Connection]struct{}
	mu            sync.Mutex
}

func (cp *ConnectionPool) AddConnection(connection *Connection) {
	defer cp.mu.Unlock()
	cp.mu.Lock()
	cp.ConnectionMap[connection] = struct{}{}

}

func (cp *ConnectionPool) DeleteConnection(connection *Connection) {
	defer cp.mu.Unlock()
	cp.mu.Lock()
	delete(cp.ConnectionMap, connection)

}

func (cp *ConnectionPool) Broadcast(buffer []byte) {
	defer cp.mu.Unlock()
	cp.mu.Lock()

	for connection := range cp.ConnectionMap {
		copy(connection.buffer, buffer)
		select {
		case connection.bufferChannel <- connection.buffer:
		default:
		}
	}
}

func NewConnectionPool() *ConnectionPool {
	connectionMap := make(map[*Connection]struct{})
	return &ConnectionPool{ConnectionMap: connectionMap}
}

func stream(connectionPool *ConnectionPool, filenames []string) {
	previousFilename := ""

	for {
		filename := filenames[rand.Intn(len(filenames))]
		for filename == previousFilename {
			filename = filenames[rand.Intn(len(filenames))]
		}

		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}

		content, err := io.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}

		buffer := make([]byte, BUFFERSIZE)
		tempfile := bytes.NewReader(content)
		ticker := time.NewTicker(time.Millisecond * DELAY)

		for range ticker.C {
			_, err := tempfile.Read(buffer)
			if err == io.EOF {
				ticker.Stop()
				break
			}
			connectionPool.Broadcast(buffer)
		}
	}
}
