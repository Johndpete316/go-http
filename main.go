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
)

// structure of an HTTP request
// inital line
// zero or more header lines
// a blank line (CRLF)
// optional message body
// ex. GET /path/to/file/index.html HTTP/1.0

// more info @ https://www.jmarshall.com/easy/http/

const PORT string = ":80"

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
}

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
	// parser is fragile,
	// assumes single space between Method, Path and Version
	// assumes all 3 are present, breaks othewise
	// splits on space for headers, breaks if spacing in headers
	// should lowercase all headers
	// needs error handling to return to handler func to return error message to client

	req := Request{
		Headers: make(map[string]string),
		Body:    "",
	}

	i := 0
	for {
		message, err := r.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			break
		}
		// fmt.Print(message)

		if message == "\r\n" {
			// read body
			sbodyLen, ok := req.Headers["Content-Length"]
			if !ok && req.Method != "POST" {
				break // Content-Length header missing, break
			} else if req.Method == "POST" {
				fmt.Println("POST should have body, return 401")
			}
			bodyLen, err := strconv.ParseInt(sbodyLen, 10, 32)
			if err != nil {
				fmt.Println(err)
				break
			} else if bodyLen <= 0 {
				break
			}

			body := make([]byte, bodyLen)
			_, err = io.ReadFull(r, body)
			if err != nil {
				break
			}
			req.Body = string(body)
			break
		}

		sMessage := strings.ReplaceAll(message, "\r\n", "")
		if i == 0 {
			s := strings.Split(sMessage, " ")

			req.Method = s[0]
			req.Path = s[1]
			req.Version = s[2]
		} else {
			s := strings.Split(sMessage, " ")
			headerKey := strings.ReplaceAll(s[0], ":", "")
			req.Headers[headerKey] = s[1]
		}

		i++
	}

	return &req, nil
}

func formatResponse(r Response) (string, error) {

	/* 		res = fmt.Sprintf(
	HTTP/1.0 200 OK\r\n
	Date: \r\n
	Content-Type: text/html\r\n
	Content-Length: %d\r\n
	\r\n
	Body

	dateHeader, bodyCount, string(body)) */
	var sb strings.Builder

	// Status line
	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", r.Version, r.ResCode, r.Reason))

	// Headers
	for k, v := range r.Headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	// Blank line to separate headers from body
	sb.WriteString("\r\n")

	// Body
	sb.WriteString(r.Body)

	return sb.String(), nil
}

func handleTCPConnections(c net.Conn) {
	defer c.Close()
	// request
	fmt.Printf("Connection started from %s\n", c.RemoteAddr().String())
	r := bufio.NewReader(c)

	req, err := parseRequest(r)
	if err != nil {
		fmt.Println(err)
	}

	var res string

	// simple router??? I think?
	if req.Path == "/" || req.Path == "/index.html" {
		body, err := os.ReadFile("index.html")
		if err != nil {
			fmt.Println(err)
			return
		}

		code := "200"
		reason, ok := statusReasons[code]
		if !ok {
			reason = "Unknown"
		}

		res, err = formatResponse(Response{
			Version: "HTTP/1.0",
			ResCode: code,
			Reason:  reason,
			Headers: map[string]string{
				"Content-Type":   "text/html",
				"Content-Length": strconv.Itoa(len(body)),
				"Date":           time.Now().Format(http.TimeFormat),
			},
			Body: string(body),
		})
		if err != nil {
			fmt.Println(err)
		}
	} else {
		body, err := os.ReadFile("not-found.html")
		if err != nil {
			fmt.Println(err)
			return
		}

		code := "404"
		reason, ok := statusReasons[code]
		if !ok {
			reason = "Unkown"
		}

		res, err = formatResponse(Response{
			Version: "HTTP/1.0",
			ResCode: code,
			Reason:  reason,
			Headers: map[string]string{
				"Content-Type":   "text/html",
				"Content-Length": strconv.Itoa(len(body)),
				"Date":           time.Now().Format(http.TimeFormat),
			},
			Body: string(body),
		})
		if err != nil {
			fmt.Println(err)
		}
	}

	// response
	// returns index.html
	c.Write([]byte(res))
}
