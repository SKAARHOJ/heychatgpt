package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/nfnt/resize"
)

const (
	serverAddr  = "192.168.11.166:9923"
	initialCmd  = "list\n"
	imageUrl    = "https://picsum.photos/536/354"
	imageWidth  = 96
	imageHeight = 64
	imageType   = 1
	bufferSize  = 5
)

type ImageBuffer struct {
	buffer  [][]byte
	current int
	mutex   sync.Mutex
}

func NewImageBuffer() *ImageBuffer {
	return &ImageBuffer{
		buffer:  make([][]byte, bufferSize),
		current: 0,
	}
}

func (ib *ImageBuffer) AddImage(image []byte) {
	ib.mutex.Lock()
	defer ib.mutex.Unlock()

	ib.buffer[ib.current] = image
	ib.current = (ib.current + 1) % bufferSize
}

func (ib *ImageBuffer) GetNextImage() []byte {
	ib.mutex.Lock()
	defer ib.mutex.Unlock()

	return ib.buffer[ib.current]
}

func main() {
	imageBuffer := NewImageBuffer()

	// Fetch initial images
	for i := 0; i < bufferSize; i++ {
		image, err := fetchAndScaleImage(imageUrl, imageWidth, imageHeight)
		if err != nil {
			fmt.Println("Error fetching or scaling image:", err)
			os.Exit(1)
		}
		imageBuffer.AddImage(image)
	}

	// Connect to the server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Error connecting to the server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Send the initial command
	_, err = conn.Write([]byte(initialCmd))
	if err != nil {
		fmt.Println("Error sending initial command:", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		command := scanner.Text()

		// Check if the command matches the expected format "HWC#X.Y=Down"
		if strings.HasSuffix(command, "=Down") {
			// Parse the command to extract HWCID
			hwcID := parseHWCID(command)
			fmt.Println(hwcID)

			// Get the next image from the buffer
			image := imageBuffer.GetNextImage()

			// Create a JSON package
			jsonData := createJSONPackage(hwcID, image)

			// Send the JSON package to the server
			_, err := conn.Write(jsonData)
			if err != nil {
				fmt.Println("Error sending JSON package:", err)
				os.Exit(1)
			}

			// Fetch and replace the used image in the buffer
			newImage, err := fetchAndScaleImage(imageUrl, imageWidth, imageHeight)
			if err != nil {
				fmt.Println("Error fetching or scaling new image:", err)
				os.Exit(1)
			}
			imageBuffer.AddImage(newImage)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from server:", err)
	}
}

func fetchAndScaleImage(url string, width, height int) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	img, _, err := image.Decode(response.Body)
	if err != nil {
		return nil, err
	}

	scaledImg := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)

	var buf []byte
	if strings.HasSuffix(url, ".jpg") || strings.HasSuffix(url, ".jpeg") {
		buf, err = encodeToJPEG(scaledImg)
	} else {
		buf, err = encodeToPNG(scaledImg)
	}

	if err != nil {
		return nil, err
	}

	return buf, nil
}

func encodeToJPEG(img image.Image) ([]byte, error) {
	var buf []byte
	buffer := bytes.NewBuffer(buf)
	err := jpeg.Encode(buffer, img, nil)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeToPNG(img image.Image) ([]byte, error) {
	var buf []byte
	buffer := bytes.NewBuffer(buf)
	err := png.Encode(buffer, img)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func parseHWCID(command string) int {
	// Split the command by '#' and '.' to extract the HWC number
	parts := strings.Split(command, "#")
	if len(parts) != 2 {
		return 0
	}

	hwcPart := parts[1]
	hwcParts := strings.Split(hwcPart, ".")
	if len(hwcParts) != 2 {
		return 0
	}

	hwcIDStr := hwcParts[0]
	hwcID, err := strconv.Atoi(hwcIDStr)
	if err != nil {
		return 0
	}

	return hwcID
}

func createJSONPackage(hwcID int, image []byte) []byte {
	jsonData := map[string]interface{}{
		"HWCIDs": []int{hwcID},
		"Processors": map[string]interface{}{
			"GfxConv": map[string]interface{}{
				"W":         imageWidth,
				"H":         imageHeight,
				"Scaling":   2,
				"ImageType": imageType,
				"ImageData": base64.StdEncoding.EncodeToString(image),
			},
		},
	}

	jsonBytes, _ := json.Marshal(jsonData)
	return append(jsonBytes, '\n')
}
