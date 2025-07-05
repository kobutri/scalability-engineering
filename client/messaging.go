package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

func getContainerIP(containerID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", err
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "", err
	}

	for _, net := range info.NetworkSettings.Networks {
		return net.IPAddress, nil
	}

	return "", fmt.Errorf("no IP address found for container %s", containerID)
}

func sendMessageToContainer(ip string, port int, message string) error {
	url := fmt.Sprintf("http://%s:%d/message", ip, port)
	resp, err := http.Post(url, "text/plain", io.NopCloser(strings.NewReader(message)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("Status:", resp.Status)
	return nil
}

func sendMessage(sender, receiver string, message string) {
	data := map[string]interface{}{
		"container_id": "sender",
		"message_id":   uuid.NewString(),
		"message":      message,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"status":       "sent",
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	receiver = "scalability-engineering-client-" + receiver // Ensure the name has the prefix
	url := fmt.Sprintf("http://%s:9090/message", receiver)

	log.Printf("Sending message to %s: %s", receiver, url)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal("error sending message:", err)
	}
	defer resp.Body.Close()

	log.Println("Status:", resp.Status)
}
