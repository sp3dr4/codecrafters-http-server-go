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

type service struct {
	directory string
}

var svc service

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

type Response struct {
	statusCode int
	statusMsg  string
	headers    []Header
	body       string
}

func (rs *Response) Write(conn *net.Conn) error {
	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\n", rs.statusCode, rs.statusMsg)

	headers := append(rs.headers, Header{name: "Content-Length", value: fmt.Sprintf("%d", len(rs.body))})
	for _, h := range headers {
		resp += fmt.Sprintf("%s: %s\r\n", h.name, h.value)
	}
	resp += "\r\n" // End headers
	resp += rs.body

	_, err := fmt.Fprint(*conn, resp)
	return err
}

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

	// if _, err = fmt.Fprint(*c, resp); err != nil {
	// 	logger.Println("Error writing response to connection: ", err.Error())
	// 	os.Exit(1)
	// }

	accEnc, ok := req.GetHeader("Accept-Encoding")
	if ok && accEnc == "gzip" {
		resp.headers = append(resp.headers, Header{name: "Content-Encoding", value: "gzip"})
	}

	if err := resp.Write(c); err != nil {
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

func handleRequest(req Request) (Response, error) {
	logger.Println("handling request: ", req)

	routes := []struct {
		pattern string
		handler func(Request) (Response, error)
	}{
		{`^\/files\/\S+$`, svc.getfile},
		{`^\/echo\/\S+$`, svc.echo},
		{`^\/user-agent$`, svc.useragent},
		{`^\/$`, svc.root},
	}

	var resp Response
	matched := false
	for _, r := range routes {
		ok, err := regexp.MatchString(r.pattern, req.reqLine[1])
		fmt.Printf("%s -> %s -> %t\n", req.reqLine[1], r.pattern, ok)
		if err != nil {
			logger.Println("Error matching path to route pattern: ", err.Error())
			continue
		}
		if ok {
			matched = true
			resp, err = r.handler(req)
			if err != nil {
				return resp, err
			}
			break
		}
	}

	if !matched {
		resp = Response{
			statusCode: 404,
			statusMsg:  "Not Found",
			headers:    []Header{},
			body:       "",
		}
	}

	return resp, nil
}

func (sv *service) echo(req Request) (Response, error) {
	toEcho := strings.Split(req.reqLine[1][1:], "/")[1]
	resp := Response{
		statusCode: 200,
		statusMsg:  "OK",
		headers: []Header{
			{name: "Content-Type", value: "text/plain"},
		},
		body: toEcho,
	}
	return resp, nil
}

func (sv *service) useragent(req Request) (Response, error) {
	uagent, ok := req.GetHeader("user-agent")
	if !ok {
		return Response{}, fmt.Errorf("User-Agent header not found")
	}
	resp := Response{
		statusCode: 200,
		statusMsg:  "OK",
		headers: []Header{
			{name: "Content-Type", value: "text/plain"},
		},
		body: uagent,
	}
	return resp, nil
}

func (sv *service) root(req Request) (Response, error) {
	resp := Response{
		statusCode: 200,
		statusMsg:  "OK",
		headers:    []Header{},
		body:       "",
	}
	return resp, nil
}

func (sv *service) getfile(req Request) (Response, error) {
	filename := strings.Split(req.reqLine[1][1:], "/")[1]
	method := strings.ToUpper(req.reqLine[0])

	resp := Response{
		statusCode: 200,
		statusMsg:  "OK",
		headers:    []Header{},
		body:       "",
	}

	if method == "GET" {
		dat, err := os.ReadFile(fmt.Sprintf("%s%s", sv.directory, filename))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				resp.statusCode = 404
				resp.statusMsg = "Not Found"
				return resp, nil
			}
			logger.Println("Error reading file: ", err.Error())
			return resp, err
		}

		datStr := string(dat)
		resp.headers = append(resp.headers, Header{name: "Content-Type", value: "application/octet-stream"})
		resp.body = datStr

		return resp, nil
	}

	if method == "POST" {
		if err := os.WriteFile(fmt.Sprintf("%s%s", sv.directory, filename), []byte(req.body), 0666); err != nil {
			return resp, err
		}
		resp.statusCode = 201
		resp.statusMsg = "Created"

		return resp, nil
	}

	return resp, fmt.Errorf("invalid method %v", req.reqLine[0])
}
