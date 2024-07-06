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

type Header struct {
	name  string
	value string
}

type Request struct {
	reqLine []string
	headers []Header
	body    string
}

func (r *Request) GetHeader(name string) (string, bool) {
	ix := slices.IndexFunc(r.headers, func(h Header) bool { return strings.ToLower(name) == h.name })
	if ix == -1 {
		return "", false
	}
	return r.headers[ix].value, true
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

func parseRequest(r *bufio.Reader) (Request, error) {
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		return Request{}, err
	}
	logger.Printf("n: %d\n", n)
	if n == 1024 {
		return Request{}, fmt.Errorf("didn't handle big request over 1024 bytes")
	}

	raw := string(buf[:n])
	// logger.Printf("raw:\n%q\n", raw)
	lines := strings.Split(raw, "\n")

	req := Request{
		reqLine: strings.Fields(lines[0]),
		headers: []Header{},
		body:    "",
	}

	if len(req.reqLine) != 3 {
		return Request{}, fmt.Errorf("expected 3 parts in request line, got %v", req.reqLine)
	}

	i := 1
	for {
		ln := strings.TrimSpace(lines[i])
		if ln == "" {
			i++
			break
		}
		elems := strings.Split(ln, ": ")
		req.headers = append(req.headers, Header{name: strings.ToLower(elems[0]), value: elems[1]})
		i++
	}

	// for j := i; j < len(lines); j++ {
	// 	logger.Printf("lines.%d: %q\n", j, lines[j])
	// }

	req.body = lines[i]

	return req, nil
}

func handleRequest(req Request) (string, error) {
	logger.Println("handling request: ", req)

	routes := []struct {
		pattern string
		handler func(Request) (string, error)
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

func (sv *service) echo(req Request) (string, error) {
	toEcho := strings.Split(req.reqLine[1][1:], "/")[1]
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho), nil
}

func (sv *service) useragent(req Request) (string, error) {
	uagent, ok := req.GetHeader("user-agent")
	if !ok {
		return "", fmt.Errorf("User-Agent header not found")
	}
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(uagent), uagent), nil
}

func (sv *service) root(req Request) (string, error) {
	return "HTTP/1.1 200 OK\r\n\r\n", nil
}

func (sv *service) getfile(req Request) (string, error) {
	filename := strings.Split(req.reqLine[1][1:], "/")[1]

	if req.reqLine[0] == "GET" {
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

	if req.reqLine[0] == "POST" {
		if err := os.WriteFile(fmt.Sprintf("%s%s", sv.directory, filename), []byte(req.body), 0666); err != nil {
			return "", err
		}
		return "HTTP/1.1 201 Created\r\n\r\n", nil
	}

	return "", fmt.Errorf("invalid method %v", req.reqLine[0])
}
