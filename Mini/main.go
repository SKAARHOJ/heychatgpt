package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// Define variables for Red, Green, and Blue gains
var RedGain, GreenGain, BlueGain int = 0, 0, 0
var luminanceControl int = 1000

func main() {
	// Connect to the Raw Panel server
	conn, err := net.Dial("tcp", "192.168.11.194:9923")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	// Initialize the connection by sending "list\n"
	_, err = conn.Write([]byte("list\n"))
	if err != nil {
		fmt.Println("Error sending initialization command:", err)
		return
	}

	// Create a scanner to read incoming messages
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		// Process incoming commands
		command := scanner.Text()
		fmt.Println("Received:", command)

		if strings.HasPrefix(command, "HWC#20=Abs:") {
			// Parse fader value from the command
			value := command[11:]
			processFader(conn, value)
		} else if strings.HasPrefix(command, "HWC#") {
			// Parse encoder and value from the command
			parts := strings.Split(command, "=")
			if len(parts) == 2 {
				encoder := parts[0][4:] // Extract the encoder number
				value := parts[1]
				processEncoder(conn, encoder, value)
			}
		}
	}
}

func processEncoder(conn net.Conn, encoder, value string) {
	// Convert the value to an integer
	pulses := 0
	_, err := fmt.Sscanf(value, "Enc:%d", &pulses)
	if err != nil {
		fmt.Println("Error parsing encoder value:", err)
		return
	}

	// Update RedGain, GreenGain, and BlueGain based on the encoder
	// You can add error checking or bounds checking here
	var displayValue int
	switch encoder {
	case "4":
		RedGain += pulses
		if RedGain < 0 {
			RedGain = 0
		}
		displayValue = RedGain
	case "5":
		GreenGain += pulses
		if GreenGain < 0 {
			GreenGain = 0
		}
		displayValue = GreenGain
	case "6":
		BlueGain += pulses
		if BlueGain < 0 {
			BlueGain = 0
		}
		displayValue = BlueGain
	}

	// Output the updated values
	fmt.Println("RedGain:", RedGain, "GreenGain:", GreenGain, "BlueGain:", BlueGain)

	// Send a command to set the value in the display
	displayCommand := fmt.Sprintf(`{"HWCIDs":[%s],"HWCText":{"Formatting":7,"Title":"Luminance","Textline1":"%d"}}`, encoder, displayValue) + "\n"
	_, err = conn.Write([]byte(displayCommand))
	if err != nil {
		fmt.Println("Error sending display command:", err)
	}

	// Send a command to set the color of the knob on the panel
	colorCommand := fmt.Sprintf(`{"HWCIDs":[%s],"HWCMode":{"State":4},"HWCColor":{"ColorRGB":{"Red":%d,"Green":%d,"Blue":%d}}}`, encoder, RedGain, GreenGain, BlueGain) + "\n"
	_, err = conn.Write([]byte(colorCommand))
	if err != nil {
		fmt.Println("Error sending color command:", err)
	}
}

func processFader(conn net.Conn, value string) {

	// Convert the value to an integer
	position := 0
	_, err := fmt.Sscanf(value, "%d", &position)
	if err != nil {
		fmt.Println("Error parsing fader value:", err)
		return
	}
	luminanceControl = position

	// Update RedGain, GreenGain, and BlueGain based on the fader position
	// You can add error checking or bounds checking here
	RedGain = (RedGain * luminanceControl) / 1000
	GreenGain = (GreenGain * luminanceControl) / 1000
	BlueGain = (BlueGain * luminanceControl) / 1000

	// Output the updated values
	fmt.Println("RedGain:", RedGain, "GreenGain:", GreenGain, "BlueGain:", BlueGain)

	// Send a command to set the color of the knob on the panel
	colorCommand := fmt.Sprintf(`{"HWCIDs":[4,5,6],"HWCMode":{"State":4},"HWCColor":{"ColorRGB":{"Red":%d,"Green":%d,"Blue":%d}}}`, RedGain, GreenGain, BlueGain) + "\n"
	_, err = conn.Write([]byte(colorCommand))
	if err != nil {
		fmt.Println("Error sending color command:", err)
	}

	// Send a command to set the value in the display
	displayCommand := fmt.Sprintf(`{"HWCIDs":[24],"HWCText":{"Formatting":7,"Title":"Luminance","Textline1":"%d"}}`, luminanceControl) + "\n"
	_, err = conn.Write([]byte(displayCommand))
	if err != nil {
		fmt.Println("Error sending display command:", err)
	}
}
