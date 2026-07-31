package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

var _ = net.Listen
var _ = os.Exit

type config struct {
	directory string
}

type Connection struct {
	socket net.Conn
	req    *http.Request
	base   string
	body   []byte
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
	for true {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting conn: ", err.Error())
			os.Exit(1)
		}
		wg.Add(1)
		go handleRequest(conn, &wg, &cfg)
	}
}

func handleRequest(socket net.Conn, wg *sync.WaitGroup, cfg *config) {
	defer socket.Close()
	defer wg.Done()
	keepAlive := true
	for keepAlive {
		reader := bufio.NewReader(socket)
		req, err := http.ReadRequest(reader)

		if err != nil {
			if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
				fmt.Println("Error reading request:", err)
			}
			break
		}

		if req.Header.Get("Connection") == "close" {
			keepAlive = false
		}

		base := "/" + path.Base(req.URL.Path)
		body, _ := io.ReadAll(req.Body)
		conn := Connection{socket, req, base, body}

		switch req.URL.Path {
		case "/":
			handleResponse(&conn, http.StatusOK, nil, nil)
		case "/echo" + base:
			handleResponse(&conn, http.StatusOK, map[string]string{
				"Content-Type": "text/plain",
			}, []byte(base[1:]))
		case "/user-agent":
			userAgent := req.Header.Get(strings.ToLower("User-Agent"))
			handleResponse(&conn, http.StatusOK, map[string]string{
				"Content-Type": "text/plain",
			}, []byte(userAgent))
		case "/files" + base:
			switch req.Method {
			case "POST":
				handlePOSTFile(&conn, cfg)
			case "GET":
				handleGetFile(&conn, cfg)
			}
		default:
			handleResponse(&conn, http.StatusNotFound, nil, nil)
		}
	}
}

func handleGetFile(conn *Connection, cfg *config) {
	fp := path.Join(cfg.directory, conn.base)
	info, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			handleResponse(conn, http.StatusNotFound, nil, nil)
			return
		}
		panic("Something broke when checking file status")
	}
	if info.Mode().IsRegular() {
		fileBytes := make([]byte, info.Size())
		file, err := os.Open(fp)
		if err != nil {
			panic("File exists but cant open it")
		}
		_, err = bufio.NewReader(file).Read(fileBytes)
		handleResponse(conn, http.StatusOK, map[string]string{
			"Content-Type": "application/octet-stream",
		}, fileBytes)
	}
}

func handlePOSTFile(conn *Connection, cfg *config) {
	fp := path.Join(cfg.directory, conn.base)
	err := os.WriteFile(fp, conn.body, 0777)
	if err != nil {
		panic("Failed to write a file")
	}
	handleResponse(conn, http.StatusCreated, nil, nil)
}

func handleResponse(conn *Connection, status int, headers map[string]string, body []byte) {
	if headers == nil {
		headers = map[string]string{}
	}

	if len(body) > 0 && strings.Contains(conn.req.Header.Get("Accept-Encoding"), "gzip") {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(body); err != nil {
			panic("Failed to gzip response body")
		}
		if err := gz.Close(); err != nil {
			panic("Failed to finalize gzip response body")
		}
		body = buf.Bytes()
		headers["Content-Encoding"] = "gzip"
	}

	if len(body) > 0 {
		headers["Content-Length"] = strconv.Itoa(len(body))
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status)))
	for header, value := range headers {
		sb.WriteString(header)
		sb.WriteString(": ")
		sb.WriteString(value)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")
	sb.Write(body)
	conn.socket.Write([]byte(sb.String()))
}
