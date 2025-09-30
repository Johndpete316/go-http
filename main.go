package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
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

func main() {
	fmt.Printf("Listening on port %s\n", PORT)

	l, err := net.Listen("tcp", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
		}
		go handleTCPConnections(c)
	}

}

func handleTCPConnections(c net.Conn) {

	// request
	fmt.Printf("Connection started from %s\n", c.RemoteAddr().String())
	r := bufio.NewReader(c)
	for {
		message, err := r.ReadString('\n')
		if err != nil {
			break
		}
		fmt.Print(message)
		if message == "\r\n" {
			break
		}
	}

	// response
	// returns index.html

	body, err := os.ReadFile("index.html")
	if err != nil {
		fmt.Println(err)
		return
	}
	bodyCount := len(body)
	dateHeader := time.Now().Format(http.TimeFormat)
	res := fmt.Sprintf(
		"HTTP/1.0 200 OK\r\n"+
			"Date: %s\r\n"+
			"Content-Type: text/html\r\n"+
			"Content-Length: %d\r\n"+
			"\r\n"+
			"%s",

		dateHeader, bodyCount, string(body))

	c.Write([]byte(res))
}
