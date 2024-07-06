package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"regexp"
)

var logger = log.New(os.Stdout, "> ", log.Ldate|log.Ltime|log.Lmicroseconds)

type Service struct {
	directory string
}

var svc Service

type Header struct {
	name  string
	value string
}

func main() {
	filedir := flag.String("directory", "/", "Filesystem directory where to search files to serve")
	flag.Parse()

	svc = Service{
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
	req, err := ParseRequest(reader)
	if err != nil {
		logger.Println("Error parsing request: ", err.Error())
		os.Exit(1)
	}

	resp, err := handleRequest(req)
	if err != nil {
		logger.Println("Error handling request: ", err.Error())
		os.Exit(1)
	}

	resp.HandleEncoding(req)

	if err := resp.Write(c); err != nil {
		logger.Println("Error writing response to connection: ", err.Error())
		os.Exit(1)
	}
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
		logger.Printf("%s -> %s -> %t\n", req.reqLine[1], r.pattern, ok)
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
