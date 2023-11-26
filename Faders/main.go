package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	faderMutex    sync.Mutex
	faderRGB      [3]int // Values for R, G, and B faders (0-1000)
	intensity     int    // Value for overall intensity fader (0-1000)
	websocketPool = make(map[*websocket.Conn]struct{})
)

func main() {
	// Initialize the fader values
	faderRGB = [3]int{0, 0, 0}
	intensity = 0

	// Start a TCP client to connect to the Raw Panel
	go startTCPClient()

	// Create a WebSocket server to communicate with the browser
	go startWebSocketServer()

	// Serve HTTP for the webpage
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `
		<!DOCTYPE html>
<html>
<head>
    <title>RGB Fader Control</title>
    <script>
        var socket = new WebSocket("ws://" + window.location.host + "/ws");
        socket.onmessage = function(event) {
            var data = JSON.parse(event.data);
            var r = data[0];
            var g = data[1];
            var b = data[2];
            var intensity = data[3];
            var color = "rgb(" + r + "," + g + "," + b + ")";
            document.body.style.backgroundColor = color;
        };

        // Capture mouse movements and send RGB values to the server
        document.addEventListener('mousemove', function(event) {
            var mouseX = event.clientX;
            var mouseY = event.clientY;
            
            // Calculate RGB values based on mouse position (0-255)
            var r = Math.floor((mouseX / window.innerWidth) * 255);
            var g = Math.floor((mouseY / window.innerHeight) * 255);
            var b = 0; // You can adjust this as needed
            
            // Send RGB values to the server
            var rgbData = [r, g, b];
            socket.send(JSON.stringify(rgbData));
        });
    </script>
</head>
<body>
    <!-- Implement your HTML content here -->
</body>
</html>
		`
		fmt.Fprint(w, html)
	})

	http.ListenAndServe(":8080", nil)
}

func startTCPClient() {
	serverAddr := "192.168.11.155:9923"

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Failed to connect to the server:", err)
		return
	}
	defer conn.Close()

	// Send the initialization command to the server
	conn.Write([]byte("list\n"))

	// Read and process fader position inputs
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading from server:", err)
			return
		}

		data := string(buf[:n])
		processFaderInput(data)
	}
}

func processFaderInput(data string) {
	// Split the input into lines
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "HWC#") {
			// Parse fader position input
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				faderID := strings.TrimPrefix(parts[0], "HWC#")
				position := strings.TrimPrefix(parts[1], "Abs:")

				// Convert faderID and position to integers
				faderNum := parseFaderID(faderID)
				faderPos := parseFaderPosition(position)

				//fmt.Println(faderNum, faderPos)

				// Update fader values
				updateFaderValue(faderNum, faderPos)
			}
		}
	}
}

func parseFaderID(faderID string) int {
	// Parse fader number from faderID (e.g., "10" from "HWC#10")
	num := strings.TrimPrefix(faderID, "#")
	faderNum, _ := strconv.Atoi(num)
	return faderNum
}

func parseFaderPosition(position string) int {
	// Parse fader position (0-1000) from the input
	pos, _ := strconv.Atoi(position)
	return pos
}

func updateFaderValue(faderNum, faderPos int) {
	// Update the corresponding fader value (R, G, B, or intensity)
	// based on faderNum and faderPos
	faderMutex.Lock()
	defer faderMutex.Unlock()

	switch faderNum {
	case 9:
		faderRGB[0] = scaleToRGB(faderPos)
	case 10:
		faderRGB[1] = scaleToRGB(faderPos)
	case 11:
		faderRGB[2] = scaleToRGB(faderPos)
	case 12:
		intensity = faderPos
	}

	fmt.Println(faderRGB)

	// Send updated values to connected WebSocket clients
	updateWebSocketClients()
}

func scaleToRGB(value int) int {
	// Scale fader position (0-1000) to RGB value (0-255)
	return (value * 255) / 1000
}

// WebSocket handling

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func startWebSocketServer() {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Println("WebSocket upgrade error:", err)
			return
		}
		defer conn.Close()

		// Add the new WebSocket connection to the pool
		websocketPool[conn] = struct{}{}

		// Handle WebSocket client messages
		for {
			// Read RGB values from the client
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("WebSocket read error:", err)
				break
			}

			var rgbData []int
			err = json.Unmarshal(msg, &rgbData)
			if err != nil {
				fmt.Println("WebSocket JSON unmarshal error:", err)
				continue
			}

			// Adjust the position of the first fader based on the received RGB values
			// You need to implement the logic for this adjustment
			adjustFirstFaderPosition(rgbData[0])

			// Send updated fader values to all connected clients
			updateWebSocketClients()
		}

		// Remove the WebSocket connection from the pool when it's closed
		delete(websocketPool, conn)
	})
}
func updateWebSocketClients() {
	for conn := range websocketPool {
		go func(conn *websocket.Conn) {
			faderMutex.Lock()
			defer faderMutex.Unlock()

			data := []int{faderRGB[0], faderRGB[1], faderRGB[2], intensity}
			err := conn.WriteJSON(data)
			if err != nil {
				fmt.Println("Error sending data to WebSocket client:", err)
			}
			fmt.Println(data)
		}(conn)
	}
}
func adjustFirstFaderPosition(r int) {
	// Scale the received RGB value (0-255) to the range 0-1000
	position := (r * 1000) / 255

	// Construct the command to set the position of the first fader (e.g., fader 9)
	command := fmt.Sprintf(`{"HWCIDs":[9],"HWCExtended":{"Interpretation":5,"Value":%d}}`, position) + "\n"

	// Send the command to the Raw Panel over the TCP connection
	sendCommandToRawPanel(command)
}

func sendCommandToRawPanel(command string) {
	// Create a TCP connection to the Raw Panel
	rawPanelAddr := "192.168.11.155:9923"
	conn, err := net.Dial("tcp", rawPanelAddr)
	if err != nil {
		fmt.Println("Failed to connect to the Raw Panel:", err)
		return
	}
	defer conn.Close()

	// Send the command to the Raw Panel
	_, err = conn.Write([]byte(command))
	if err != nil {
		fmt.Println("Error sending command to Raw Panel:", err)
	}
}
