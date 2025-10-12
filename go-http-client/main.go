package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const USERAGENT string = "MyGoClient/1.0"

func main() {
	uriPtr := flag.String("URI", "http://localhost", "Enter the target URI")
	testPtr := flag.String("test", "basic-get", "select which test to run")
	flag.Parse()

	switch *testPtr {
	case "basic-get":

		fmt.Printf("Selected Test: %s against %s\n", *testPtr, *uriPtr)
		httpGETRequest(*uriPtr)

	case "basic-head":
		fmt.Printf("Selected Test: %s against %s\n", *testPtr, *uriPtr)
		httpHEADRequest(*uriPtr)

	case "basic-put":
		fmt.Printf("Selected Test: %s against %s\n", *testPtr, *uriPtr)
		httpPUTRequest(*uriPtr)
	}
}

func httpGETRequest(TargetURI string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, TargetURI, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}
	req.Header.Add("User-Agent", USERAGENT)
	req.Header.Add("Code-Breaker", "Ooga:Booga:1")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error perfomring GET request: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	sBody := string(body)

	defer resp.Body.Close()

	fmt.Printf("Status code: %d\n", resp.StatusCode)
	fmt.Printf("Content-Lenght: %d\n", resp.ContentLength)
	fmt.Printf("Headers: %v", resp.Header)
	fmt.Printf("Body: \n%s", sBody)
}

func httpHEADRequest(TargetURI string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodHead, TargetURI, nil)
	if err != nil {
		log.Fatalf("Error creating new request %v", err)
	}
	req.Header.Add("User-Agent", USERAGENT)
	req.Header.Add("Code-Breaker", "Ooga:Booga:1")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error performing HEAD request: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading body: %v", err)
	}
	if len(body) == int(resp.ContentLength) && resp.ContentLength > 0 {
		fmt.Println("HEAD Request returning content, not headers.")
		fmt.Printf("Body Length: %d\n", len(body))
	} else if len(body) == int(resp.ContentLength) && resp.ContentLength == 0 {
		fmt.Println("Server header processing incorrect. Content-Length is 0")
		fmt.Printf("Content-Length: %d\n", resp.ContentLength)
		fmt.Printf("Body: %d\n", len(body))
	} else if len(body) > 0 {
		fmt.Println("HEAD Request returning some content in body.")
		fmt.Printf("Body Length: %d\n", len(body))
	} else {
		fmt.Println("HEAD Request response is correct (no body).")
	}

	fmt.Printf("Status code: %d\n", resp.StatusCode)
	fmt.Printf("Content-Lenght: %d\n", resp.ContentLength)
	fmt.Printf("Headers: %v", resp.Header)
}

func httpPUTRequest(TargetURI string) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	payload := map[string]any{
		"id":   1,
		"test": "data",
		"date": time.Now().Format(http.TimeFormat),
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, TargetURI, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading body: %v", err)
	}
	fmt.Printf("Response: %v\n", resp.StatusCode)
	fmt.Printf("Headers: %v\n", resp.Header)
	fmt.Printf("Body: \n%s", string(body))
}
