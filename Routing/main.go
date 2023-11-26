package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

var mu sync.Mutex
var currentInputForOutput3 int
var receivingVideoRoutingInfo bool
var rawPanelConn net.Conn
var inputLabels [16]string // Assuming 16 inputs, adjust as needed

func main() {
	// Connect to the Raw Panel Server
	var err error
	rawPanelConn, err = net.Dial("tcp", "192.168.11.5:9973")
	if err != nil {
		fmt.Println("Failed to connect to Raw Panel Server:", err)
		return
	}
	defer rawPanelConn.Close()

	// Connect to the Video Hub
	videoHubConn, err := net.Dial("tcp", "192.168.10.61:9990")
	if err != nil {
		fmt.Println("Failed to connect to Video Hub:", err)
		return
	}
	defer videoHubConn.Close()

	// Send initial command to Raw Panel Server
	rawPanelConn.Write([]byte("list\n"))
	rawPanelConn.Write([]byte("Clear\n"))

	// Create a goroutine to continuously read and update the current input and labels
	go func() {
		scanner := bufio.NewScanner(videoHubConn)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "VIDEO OUTPUT ROUTING:") {
				// Set the flag to indicate that we are receiving video routing info
				receivingVideoRoutingInfo = true
			} else if strings.HasPrefix(line, "INPUT LABELS:") {
				// Parse input labels
				for i := 0; i < 16; i++ { // Assuming 16 inputs, adjust as needed
					if scanner.Scan() {
						label := scanner.Text()
						inputLabels[i] = label
						updateInputLabel(i, label)
					}
				}
			} else if receivingVideoRoutingInfo {
				// Find the line starting with "2" and extract the input number
				parts := strings.Fields(line)
				if len(parts) == 2 && parts[0] == "2" {
					inputNumber := parts[1]
					// Convert the input number to an integer
					input, err := strconv.Atoi(inputNumber)
					if err != nil {
						fmt.Println("Error converting input number:", err)
						continue
					}

					// Update the global variable with the current input for output 3
					mu.Lock()
					previousInput := currentInputForOutput3
					currentInputForOutput3 = input
					mu.Unlock()

					// Output the current input to the console
					fmt.Printf("Current Input for Output 3: %d\n", input)

					// Turn off the previous input
					//if previousInput != 0 {
					turnOffInput(previousInput + 1)
					//}

					// Turn on the new input
					turnOnInput(input + 1)
				} else {
					// Reset the flag when encountering another header
					receivingVideoRoutingInfo = false
				}
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading from Video Hub:", err)
		}
	}()

	// Create a scanner to read from the Raw Panel Server
	scanner := bufio.NewScanner(rawPanelConn)
	for scanner.Scan() {
		command := scanner.Text()
		if strings.HasSuffix(command, "=Down") {
			// Extract the button number from the command
			parts := strings.Split(command, "=")
			buttonNumber := parts[0][4:]

			// Convert the button number to input number (1-8 to 0-7)
			inputNumber := fmt.Sprintf("%d", (int(buttonNumber[0]) - '0' - 1))

			fmt.Println(inputNumber)

			// Prepare the command to send to the Video Hub
			videoHubCommand := fmt.Sprintf("VIDEO OUTPUT ROUTING:\n2 %s\n\n", inputNumber)

			// Send the command to the Video Hub
			_, err := videoHubConn.Write([]byte(videoHubCommand))
			if err != nil {
				fmt.Println("Failed to send command to Video Hub:", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from Raw Panel Server:", err)
	}
}

func turnOnInput(inputNumber int) {
	inputCommand := map[string]interface{}{
		"HWCIDs":      []int{inputNumber},
		"HWCMode":     map[string]interface{}{"State": 4},
		"HWCColor":    map[string]interface{}{"ColorIndex": map[string]interface{}{"Index": 4}},
		"HWCExtended": map[string]interface{}{},
		"HWCText":     map[string]interface{}{"Formatting": 7},
	}

	commandJSON, err := json.Marshal(inputCommand)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	// Send the JSON command to the Raw Panel Server
	_, err = rawPanelConn.Write(commandJSON)
	_, err = rawPanelConn.Write([]byte("\n"))
	if err != nil {
		fmt.Println("Failed to send command to Raw Panel Server:", err)
	}
}

func turnOffInput(inputNumber int) {
	inputCommand := map[string]interface{}{
		"HWCIDs":      []int{inputNumber},
		"HWCMode":     map[string]interface{}{"State": 0},
		"HWCColor":    map[string]interface{}{"ColorIndex": map[string]interface{}{"Index": 4}},
		"HWCExtended": map[string]interface{}{},
		"HWCText":     map[string]interface{}{"Formatting": 7},
	}

	commandJSON, err := json.Marshal(inputCommand)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	// Send the JSON command to the Raw Panel Server
	_, err = rawPanelConn.Write(commandJSON)
	_, err = rawPanelConn.Write([]byte("\n"))
	if err != nil {
		fmt.Println("Failed to send command to Raw Panel Server:", err)
	}
}

func updateInputLabel(inputNumber int, label string) {
	// Construct the JSON command to update the input label
	inputCommand := map[string]interface{}{
		"HWCIDs": []int{inputNumber},
		"HWCText": map[string]interface{}{
			"Formatting": 7,
			"Textline1":  "INPUT " + label,
		},
	}

	commandJSON, err := json.Marshal(inputCommand)
	if err != nil {
		fmt.Println("Error encoding JSON:", err)
		return
	}

	// Send the JSON command to the Raw Panel Server
	_, err = rawPanelConn.Write(commandJSON)
	_, err = rawPanelConn.Write([]byte("\n"))

	if err != nil {
		fmt.Println("Failed to send command to Raw Panel Server:", err)
	}
}
