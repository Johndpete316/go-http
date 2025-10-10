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

// TODO: config file so I can bring back my dream of the teapot page returning 418
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
	Body    []byte
}

var statusReasons = map[string]string{
	"200": "OK",
	"404": "Not Found",
	"400": "Bad Request",
	"401": "Unauthorized",
	"403": "Forbidden",
	"500": "Internal Server Error",
	"418": "I'm a teapot",
}

var MIMETYPES = map[string]string{
	".bin":  "application/octet-stream",
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".js":   "text/javascript",
	".json": "application/json",
	".php":  "application/x-httpd-php",
	".md":   "text/markdown",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".txt":  "text/plain",
	".webp": "image/webp",
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

func sanitizePath(userPath string, documentRoot string) (string, error) {
	if strings.Contains(userPath, "..") {
		return "", fmt.Errorf("path traversal attempt detected")
	}
	var safePath string
	cleanedPath := filepath.Clean(userPath)
	joinedPath := filepath.Join(documentRoot, cleanedPath)
	absPath, err := filepath.Abs(joinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	absDocumentRoot, err := filepath.Abs(documentRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get document root absolute path %w", err)
	}
	if strings.HasPrefix(absPath, absDocumentRoot) {
		// TODO: re-address this
		if absPath == absDocumentRoot || absPath[len(absDocumentRoot)] == filepath.Separator {
			safePath = absPath
		} else {
			return "", fmt.Errorf("invalid path")
		}
	}
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

func formatResponse(r Response) []byte {

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s %s\r\n", r.Version, r.ResCode, r.Reason))
	for k, v := range r.Headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")

	bRes := append([]byte(sb.String()), r.Body...)

	return bRes
}

func buildResponse(mimeType string, body []byte, statusCode string) Response {
	reason, ok := statusReasons[statusCode]
	if !ok {
		reason = "UNKNOWN"
	}
	resp := Response{
		Version: VERSION,
		ResCode: statusCode,
		Reason:  reason,
		Headers: map[string]string{
			"Content-Length": strconv.Itoa(len(body)),
			"Date":           time.Now().Format(http.TimeFormat),
		},
		Body: body,
	}
	if mimeType == "" {
		resp.Headers["Content-Type"] = "application/octet-stream"
		return resp
	} else {
		resp.Headers["Content-Type"] = mimeType
		return resp
	}
}

func getMimeType(filePath string) string {
	fileExt := filepath.Ext(filePath)
	mimeType, ok := MIMETYPES[fileExt]
	if !ok {
		return ""
	}
	return mimeType
}

func serveFile(filePath string, statusCode string) (Response, error) {

	var resp Response

	mimeType := getMimeType(filePath)

	body, err := os.ReadFile(filePath)
	if err != nil {
		// pass path string to build response
		return Response{}, fmt.Errorf("failed to read file %w", err)
	}
	resp = buildResponse(mimeType, body, statusCode)

	return resp, nil
}

func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}

func serveNotFound() []byte {
	notFoundHtml := filepath.Join(DOCUMENTROOT, "not-found.html")
	if fileExists(notFoundHtml) {
		var resp Response
		resp, err := serveFile(notFoundHtml, "404")
		if err != nil {
			// TODO: error logging
			resp = buildResponse("text/plain", []byte("Internal Server Error"), "500")
		}
		res := formatResponse(resp)
		return res
	} else {
		resp := buildResponse("text/plain", []byte("Not Found"), "404")
		res := formatResponse(resp)
		return res
	}
}

func handleTCPConnections(c net.Conn) {
	defer c.Close()

	// request
	fmt.Printf("Connection started from %s\n", c.RemoteAddr().String())
	r := bufio.NewReader(c)

	req, err := parseRequest(r)
	if err != nil {
		body := []byte("Bad Request")
		resp := buildResponse("text/plain", body, "400")
		res := formatResponse(resp)
		c.Write(res)
		return
	}

	sanitizedPath, err := sanitizePath(req.Path, DOCUMENTROOT)
	if err != nil {
		// TOMFOOLERY
		body := []byte("Forbidden")
		resp := buildResponse("text/plain", body, "403")
		res := formatResponse(resp)
		c.Write(res)
		return
	}
	fileStats, err := os.Stat(sanitizedPath)
	if err != nil {
		res := serveNotFound()
		c.Write(res)
		return
	}
	if fileStats.IsDir() {
		indexHtmlPath := filepath.Join(sanitizedPath, "index.html")
		var resp Response
		if !fileExists(indexHtmlPath) {
			res := serveNotFound()
			c.Write(res)
			return
		}
		resp, err = serveFile(indexHtmlPath, "200")
		if err != nil {
			resp = buildResponse("text/plain", []byte("Internal Server Error"), "500")
		}
		res := formatResponse(resp)
		c.Write(res)
		return
	} else {
		// TODO: impliment extensionless routes for html and PHP
		var resp Response
		resp, err = serveFile(sanitizedPath, "200")
		if err != nil {
			resp = buildResponse("text/plain", []byte("Internal Server Error"), "500")
		}
		res := formatResponse(resp)
		c.Write(res)
		return
	}
}
