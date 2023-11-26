package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

const (
	frameShotProAddr = "192.168.11.166:9923"
	lampAPIAddr      = "http://192.168.10.252/netio.json"
	lampUsername     = "netio"
	lampPassword     = "netio"
)

func main() {
	// Connect to Frame Shot Pro
	conn, err := net.Dial("tcp", frameShotProAddr)
	if err != nil {
		fmt.Println("Error connecting to Frame Shot Pro:", err)
		return
	}
	defer conn.Close()

	// Send "list\n" to initialize the connection
	_, err = conn.Write([]byte("list\n"))
	if err != nil {
		fmt.Println("Error sending 'list' to Frame Shot Pro:", err)
		return
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()

		// Check if the message is one of the expected patterns
		if message == "HWC#1.4=Down" {
			// Toggle the lamp
			toggleLamp()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from Frame Shot Pro:", err)
	}
}

func toggleLamp() {
	// Create JSON payload
	payload := map[string]interface{}{
		"Outputs": []map[string]interface{}{
			{
				"ID":     1,
				"Action": 4,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error encoding JSON payload:", err)
		return
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", lampAPIAddr, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating HTTP request:", err)
		return
	}

	req.SetBasicAuth(lampUsername, lampPassword)
	req.Header.Set("Content-Type", "application/json")

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending HTTP request to toggle lamp:", err)
		return
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error toggling lamp. Status code:", resp.StatusCode)
		return
	}

	fmt.Println("Lamp toggled successfully!")
}
