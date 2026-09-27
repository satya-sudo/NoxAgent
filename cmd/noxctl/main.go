package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) == 1 {
		runInteractive()
		return
	}
	if err := sendCommand(strings.Join(os.Args[1:], " ")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInteractive() {
	fmt.Println("Nox text console. Type 'exit' to quit.")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("nox> ")
		if !scanner.Scan() {
			return
		}
		command := strings.TrimSpace(scanner.Text())
		if command == "" {
			continue
		}
		if command == "exit" || command == "quit" {
			return
		}
		if err := sendCommand(command); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func sendCommand(command string) error {
	payload, _ := json.Marshal(map[string]string{"text": command})
	client := &http.Client{Timeout: 20 * time.Second}
	address := os.Getenv("NOX_ADDRESS")
	if address == "" {
		address = "127.0.0.1:7080"
	}
	response, err := client.Post("http://"+address+"/v1/commands", "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Nox is unavailable: %w", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		return fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	var reply struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return fmt.Errorf("invalid response: %w", err)
	}
	fmt.Println(reply.Message)
	return nil
}
