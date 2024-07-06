package main

import (
	"bufio"
	"fmt"
	"slices"
	"strings"
)

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

func ParseRequest(r *bufio.Reader) (Request, error) {
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
