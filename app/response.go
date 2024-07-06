package main

import (
	"fmt"
	"net"
	"slices"
	"strings"
)

type Response struct {
	statusCode int
	statusMsg  string
	headers    []Header
	body       string
}

func (rs *Response) HandleEncoding(req Request) {
	accEnc, ok := req.GetHeader("Accept-Encoding")
	if ok {
		encodings := strings.Split(accEnc, ",")
		if slices.ContainsFunc(encodings, func(e string) bool { return strings.TrimSpace(e) == "gzip" }) {
			gzipHeader := Header{name: "Content-Encoding", value: "gzip"}
			rs.headers = append(rs.headers, gzipHeader)
		}
	}
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
