package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

// should be configurable by config file eventually
const PORT string = ":80"
const VERSION string = "HTTP/1.0"
const DOCUMENTROOT string = "./public"

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

// type requestHandler func(Request) Response

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

func sanitizePath(userPath string, documentRoot string) (string, error) {

	// TODO: Re-arange this file so that path traversal attempts can be detected and rejected
	// prior to the filepath.Clean function is ran, scumbags need a 403 xD

	var safePath string

	cleanedPath := filepath.Clean(userPath)
	joinedPath := filepath.Join(documentRoot, cleanedPath)

	absPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	absDocumentRoot, err := filepath.Abs(documentRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	if strings.HasPrefix(absPath, absDocumentRoot) {
		// TODO: validate next char after prefix is a path seperator
		safePath = absPath
	} else {
		return "", fmt.Errorf("path traversal attempt")
	}
	fmt.Printf("absolute path: %s\n", absPath)

	return safePath, nil
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
			if len(s) < 3 {
				return nil, fmt.Errorf("malformed request line")
			}
			req.Method = s[0]
			req.Path = s[1]
			req.Version = s[2]

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

func formatResponse(r Response) string {

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", r.Version, r.ResCode, r.Reason))
	for k, v := range r.Headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")
	sb.WriteString(r.Body)

	return sb.String()
}

func buildResponse(body string, statusCode string) Response {
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
	return resp
}

func serveFile(filename string, statusCode string) Response {

	var resp Response

	body, err := os.ReadFile(filename)
	if err != nil {
		resp = buildResponse("Internal Server Error", "500")
		return resp
	}
	resp = buildResponse(string(body), statusCode)

	return resp
}

func handleTCPConnections(c net.Conn) {
	defer c.Close()

	var isDir bool

	// request
	fmt.Printf("Connection started from %s\n", c.RemoteAddr().String())
	r := bufio.NewReader(c)

	req, err := parseRequest(r)
	if err != nil {
		body := "Bad Request"
		resp := buildResponse(body, "400")

		formattedResp := formatResponse(resp)
		c.Write([]byte(formattedResp))
		return
	}

	// TODO: MIME Type detection, adjust Content-Type header based off this.
	// TODO: handle file not found  (404)
	// TODO: handle DIR not found  (404)
	// TODO: run through full test suite --
	// GET / → should serve index.html
	// GET /about.html → should serve about.html
	// GET /images/cat.jpg → should serve the image with correct Content-Type
	// GET /../../../etc/passwd → should return 403
	// GET /nonexistent.html → should return 404
	// TODO: Remove routes map comment block
	// TODO: config file so I can bring back my dream of the teapot page returning 418

	sanitizedPath, _ := sanitizePath(req.Path, DOCUMENTROOT)
	fmt.Println(sanitizedPath)
	fileStats, err := os.Stat(sanitizedPath)
	if err != nil {
		isDir = false
	} else {
		isDir = fileStats.IsDir()
	}
	if isDir {
		resp := serveFile(sanitizedPath+"/index.html", "200")
		res := formatResponse(resp)
		c.Write([]byte(res))
		return
	} else {
		resp := serveFile(sanitizedPath+".html", "200")
		res := formatResponse(resp)
		c.Write([]byte(res))
		return
	}

	/* 	var routes = map[string]requestHandler{
	   		"/": func(req Request) Response {
	   			resp := serveFile("index.html", "200")
	   			return resp
	   		},
	   		"/abcd": func(req Request) Response {
	   			resp := serveFile("abcd.html", "200")
	   			return resp
	   		},
	   		"/418": func(req Request) Response {
	   			resp := serveFile("teapot.html", "418")
	   			return resp
	   		},
	   	}

	   	if handler, exists := routes[req.Path]; exists {
	   		resp := handler(*req)
	   		res := formatResponse(resp)

	   		c.Write([]byte(res))
	   		return
	   	} else {
	   		resp := serveFile("not-found.html", "404")
	   		res := formatResponse(resp)

	   		c.Write([]byte(res))
	   		return
	   	} */

}
