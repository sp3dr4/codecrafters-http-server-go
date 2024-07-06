package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"regexp"
	"slices"
	"strings"
)

var logger = log.New(os.Stdout, "> ", log.Ldate|log.Ltime|log.Lmicroseconds)

type request struct {
	reqLine []string
	headers []string
}

type service struct {
	directory string
}

var svc service

func main() {
	filedir := flag.String("directory", "/", "Filesystem directory where to search files to serve")
	flag.Parse()

	svc = service{
		directory: *filedir,
	}

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		logger.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			logger.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleConn(&conn)
	}
}

func handleConn(c *net.Conn) {
	defer (*c).Close()
	reader := bufio.NewReader(*c)
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

	if _, err = fmt.Fprint(*c, resp); err != nil {
		logger.Println("Error writing response to connection: ", err.Error())
		os.Exit(1)
	}
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

	if len(req.reqLine) != 3 {
		return request{}, fmt.Errorf("expected 3 parts in request line, got %v", req.reqLine)
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

	routes := []struct {
		pattern string
		handler func(request) (string, error)
	}{
		{`^\/files\/\S+$`, svc.getfile},
		{`^\/echo\/\S+$`, svc.echo},
		{`^\/user-agent$`, svc.useragent},
		{`^\/$`, svc.root},
	}

	resp := ""
	for _, r := range routes {
		ok, err := regexp.MatchString(r.pattern, req.reqLine[1])
		fmt.Printf("%s -> %s -> %t\n", req.reqLine[1], r.pattern, ok)
		if err != nil {
			logger.Println("Error matching path to route pattern: ", err.Error())
			continue
		}
		if ok {
			resp, err = r.handler(req)
			if err != nil {
				return "", err
			}
			break
		}
	}

	if resp == "" {
		resp = "HTTP/1.1 404 Not Found\r\n\r\n"
	}

	return resp, nil
}

func (sv *service) echo(req request) (string, error) {
	toEcho := strings.Split(req.reqLine[1][1:], "/")[1]
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho), nil
}

func (sv *service) useragent(req request) (string, error) {
	agentIx := slices.IndexFunc(req.headers, func(h string) bool { return strings.HasPrefix(strings.ToLower(h), "user-agent: ") })
	if agentIx == -1 {
		return "", fmt.Errorf("user-agent header not found")
	}
	agent := strings.Split(req.headers[agentIx], ": ")[1]
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(agent), agent), nil
}

func (sv *service) root(req request) (string, error) {
	return "HTTP/1.1 200 OK\r\n\r\n", nil
}

func (sv *service) getfile(req request) (string, error) {
	filename := strings.Split(req.reqLine[1][1:], "/")[1]

	dat, err := os.ReadFile(fmt.Sprintf("%s%s", sv.directory, filename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "HTTP/1.1 404 Not Found\r\n\r\n", nil
		}
		logger.Println("Error reading file: ", err.Error())
		return "", err
	}

	datStr := string(dat)
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", len(datStr), datStr), nil
}
