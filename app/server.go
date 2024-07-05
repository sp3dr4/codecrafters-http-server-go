package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
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
	req, err := parseRequest(reader)
	if err != nil {
		logger.Println("Error parsing request: ", err.Error())
		os.Exit(1)
	}

	resp, err := handleRequest(req)
	if err != nil {
		logger.Println("Error handling request: ", err.Error())
		os.Exit(1)
	}

	if _, err = fmt.Fprint(conn, resp); err != nil {
		logger.Println("Error writing response to connection: ", err.Error())
		os.Exit(1)
	}
}

type request struct {
	reqLine []string
	headers []string
}

func parseRequest(r *bufio.Reader) (request, error) {
	buf := make([]byte, 1024)
	_, err := r.Read(buf)
	if err != nil {
		return request{}, err
	}
	// logger.Printf("n: %d\n", n)

	raw := string(buf)
	lines := strings.Split(raw, "\n")

	req := request{
		reqLine: strings.Fields(lines[0]),
		headers: []string{},
	}

	i := 1
	for {
		ln := strings.TrimSpace(lines[i])
		if ln == "" {
			break
		}
		req.headers = append(req.headers, ln)
		i++
	}

	return req, nil
}

func handleRequest(req request) (string, error) {
	logger.Println("handling request: ", req)

	line := req.reqLine
	if len(line) != 3 {
		return "", fmt.Errorf("expected 3 parts in request line, got %v", line)
	}

	resp := "HTTP/1.1 404 Not Found\r\n\r\n"

	if strings.HasPrefix(line[1], "/echo/") {
		toEcho := strings.Split(line[1][1:], "/")[1]
		resp = fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho)
	} else if line[1] == "/user-agent" {
		agentIx := slices.IndexFunc(req.headers, func(h string) bool { return strings.HasPrefix(strings.ToLower(h), "user-agent: ") })
		if agentIx == -1 {
			return "", fmt.Errorf("user-agent header not found")
		}
		agent := strings.Split(req.headers[agentIx], ": ")[1]
		resp = fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(agent), agent)
	} else if line[1] == "/" {
		resp = "HTTP/1.1 200 OK\r\n\r\n"
	}

	return resp, nil
}
