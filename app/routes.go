package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

func (sv *Service) echo(req Request) (Response, error) {
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

func (sv *Service) useragent(req Request) (Response, error) {
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

func (sv *Service) root(req Request) (Response, error) {
	resp := Response{
		statusCode: 200,
		statusMsg:  "OK",
		headers:    []Header{},
		body:       "",
	}
	return resp, nil
}

func (sv *Service) getfile(req Request) (Response, error) {
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
