package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

var logger = log.New(os.Stdout, "> ", log.Ldate|log.Ltime|log.Lmicroseconds)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		logger.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	conn, err := l.Accept()
	if err != nil {
		logger.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}

	reader := bufio.NewReader(conn)
	reqLine, err := parseRequest(reader)
	if err != nil {
		logger.Println("Error parsing request: ", err.Error())
		os.Exit(1)
	}
	logger.Printf("request line: %v\n", reqLine)

	resp, err := handleRequest(reqLine)
	if err != nil {
		logger.Println("Error handling request: ", err.Error())
		os.Exit(1)
	}

	if _, err = fmt.Fprint(conn, resp); err != nil {
		logger.Println("Error writing response to connection: ", err.Error())
		os.Exit(1)
	}
}

func parseRequest(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}

	parts := strings.Fields(line)

	return parts, nil
}

func handleRequest(line []string) (string, error) {
	if len(line) != 3 {
		return "", fmt.Errorf("expected 3 parts in request line, got %v", line)
	}

	resp := "HTTP/1.1 404 Not Found\r\n\r\n"

	if strings.HasPrefix(line[1], "/echo/") {
		toEcho := strings.Split(line[1][1:], "/")[1]
		resp = fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho)
	} else if line[1] == "/" {
		resp = "HTTP/1.1 200 OK\r\n\r\n"
	}

	return resp, nil
}
