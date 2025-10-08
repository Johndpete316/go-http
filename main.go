package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// structure of an HTTP request
// inital line
// zero or more header lines
// a blank line (CRLF)
// optional message body
// ex. GET /path/to/file/index.html HTTP/1.0

// more info @ https://www.jmarshall.com/easy/http/

const PORT string = ":80"
const VERSION string = "HTTP/1.0"

type Request struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string
	Body    string
}

type Response struct {
	Version string
	ResCode string
	Reason  string
	Headers map[string]string
	Body    string
}

var statusReasons = map[string]string{
	"200": "OK",
	"404": "Not Found",
	"400": "Bad Request",
	"401": "Unauthorized",
	"500": "Internal Server Error",
	"418": "I'm a teapot",
}

type requestHandler func(Request) Response

func main() {
	fmt.Printf("Listening on port %s\n", PORT)

	l, err := net.Listen("tcp", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
		}
		go handleTCPConnections(c)
	}

}

func parseRequest(r *bufio.Reader) (*Request, error) {

	req := Request{
		Headers: make(map[string]string),
		Body:    "",
	}

	i := 0
	for {
		message, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read the request line: %w", err)
		}

		if message == "\r\n" {
			// read body
			sbodyLen, ok := req.Headers["Content-Length"]
			if !ok && req.Method != "POST" {
				break // Content-Length header missing, break
			} else if req.Method == "POST" {
				return nil, fmt.Errorf("POST missing Content-Length")
			}
			bodyLen, err := strconv.ParseInt(sbodyLen, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("failed to parse body length: %w", err)
			} else if bodyLen <= 0 {
				break
			}

			body := make([]byte, bodyLen)
			_, err = io.ReadFull(r, body)
			if err != nil {
				return nil, fmt.Errorf("failed to read the body: %w", err)
			}
			req.Body = string(body)
			break
		}

		sMessage := strings.ReplaceAll(message, "\r\n", "")
		if i == 0 {

			s := strings.Fields(sMessage)

			req.Method = s[0]
			req.Path = s[1]
			req.Version = s[2]

			if len(s) < 3 {
				return nil, fmt.Errorf("malformed request line")
			}
		} else {
			s := strings.SplitN(strings.ToLower(sMessage), ":", 2)
			if len(s) == 2 {
				headerKey := s[0]
				req.Headers[headerKey] = strings.TrimLeftFunc(s[1], unicode.IsSpace)

			} else {
				return nil, fmt.Errorf("malformed headers")
			}
		}

		i++
	}

	return &req, nil
}

func formatResponse(r Response) (string, error) {

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", r.Version, r.ResCode, r.Reason))
	for k, v := range r.Headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")
	_, err := sb.WriteString(r.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write body: %w", err)
	}

	return sb.String(), nil
}

func serveFile(filename string, statusCode string) (Response, error) {
	body, err := os.ReadFile(filename)
	if err != nil {
		return Response{}, fmt.Errorf("failed to read file %w", err)
	}

	reason, ok := statusReasons[statusCode]
	if !ok {
		reason = "UNKNOWN"
	}

	resp := Response{
		Version: VERSION,
		ResCode: statusCode,
		Reason:  reason,
		Headers: map[string]string{
			"Content-Type":   "text/html",
			"Content-Length": strconv.Itoa(len(body)),
			"Date":           time.Now().Format(http.TimeFormat),
		},
		Body: string(body),
	}

	return resp, nil

}

func handleTCPConnections(c net.Conn) {
	defer c.Close()
	// request
	fmt.Printf("Connection started from %s\n", c.RemoteAddr().String())
	r := bufio.NewReader(c)

	req, err := parseRequest(r)
	if err != nil {
		resp := Response{
			Version: VERSION,
			ResCode: "400",
			Reason:  "Bad Request",
			Headers: map[string]string{
				"Date":           time.Now().Format(http.TimeFormat),
				"Content-Length": strconv.Itoa(len(err.Error())),
			},
			Body: err.Error(),
		}

		formattedResp, _ := formatResponse(resp)
		c.Write([]byte(formattedResp))
		return
	}

	var routes = map[string]requestHandler{
		"/": func(req Request) Response {
			resp, _ := serveFile("index.html", "200")
			return resp
		},
		"/abcd": func(req Request) Response {
			resp, _ := serveFile("abcd.html", "200")
			return resp
		},
		"/418": func(req Request) Response {
			resp, _ := serveFile("teapot.html", "418")
			return resp
		},
	}

	if handler, exists := routes[req.Path]; exists {
		resp := handler(*req)
		res, _ := formatResponse(resp)

		c.Write([]byte(res))
		return
	} else {
		resp, _ := serveFile("not-found.html", "404")
		res, _ := formatResponse(resp)

		c.Write([]byte(res))
		return
	}

}
