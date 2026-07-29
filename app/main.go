package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

var _ = net.Listen
var _ = os.Exit

type config struct {
	directory string
}

func parseConfig() config {
	cfg := config{}
	flag.StringVar(&cfg.directory, "directory", "./", "Root directory path")
	flag.Parse()
	return cfg
}

func main() {
	cfg := parseConfig()

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()
	wg := sync.WaitGroup{}
	timeout := time.NewTimer(10 * time.Second)
	for true {
		select {
		case <-timeout.C:
			os.Exit(1)
		default:
			conn, err := l.Accept()
			if err != nil {
				fmt.Println("Error accepting connection: ", err.Error())
				os.Exit(1)
			}
			wg.Add(1)
			go handleRequest(conn, &wg, &cfg)
			timeout.Reset(10 * time.Second)
		}
	}
}

func handleRequest(conn net.Conn, wg *sync.WaitGroup, cfg *config) {
	defer conn.Close()
	defer wg.Done()

	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		os.Exit(1)
	}
	base := "/" + path.Base(req.URL.Path)
	switch req.URL.Path {
	case "/":
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	case "/echo" + base:
		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(base)-1, base[1:])))
	case "/user-agent":
		userAgent := req.Header.Get(strings.ToLower("User-Agent"))
		conn.Write([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(userAgent), userAgent)))
	case "/files" + base:
		handleFiles(conn, base, cfg)
	default:
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}

func handleFiles(conn net.Conn, base string, cfg *config) {
	
}
