package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type ClientContainer struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	HostPort  string `json:"host_port"`
	WebURL    string `json:"web_url"`
}

type ClientManager struct {
	dockerClient *client.Client
	projectName  string
	composeFile  string
	workingDir   string
	project      *types.Project
}

type NamesData struct {
	Version   int      `json:"version"`
	Timestamp string   `json:"timestamp"`
	Elements  []string `json:"elements"`
}

func NewClientManager(projectName, composeFile, workingDir string) (*ClientManager, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Load the compose project
	project, err := loadComposeProject(composeFile, projectName)
	if err != nil {
		log.Printf("Warning: Failed to load compose project: %v", err)
		// Continue without compose project for basic container management
	}

	return &ClientManager{
		dockerClient: dockerClient,
		projectName:  projectName,
		composeFile:  composeFile,
		workingDir:   workingDir,
		project:      project,
	}, nil
}

func (cm *ClientManager) Close() error {
	return cm.dockerClient.Close()
}

// loadComposeProject loads the compose project configuration
func loadComposeProject(composeFile, projectName string) (*types.Project, error) {
	options, err := cli.NewProjectOptions(
		[]string{composeFile},
		cli.WithOsEnv,
		cli.WithDotEnv,
		cli.WithName(projectName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project options: %w", err)
	}

	project, err := options.LoadProject(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load project: %w", err)
	}

	return project, nil
}

// LoadNamesFromFile loads names from the names.json file
func (cm *ClientManager) LoadNamesFromFile() ([]string, error) {
	namesFile := filepath.Join(cm.workingDir, "..", "data", "names.json")
	data, err := os.ReadFile(namesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read names file: %w", err)
	}

	var namesData NamesData
	if err := json.Unmarshal(data, &namesData); err != nil {
		return nil, fmt.Errorf("failed to parse names file: %w", err)
	}

	return namesData.Elements, nil
}

// GenerateRandomName generates a random client name
func (cm *ClientManager) GenerateRandomName() (string, error) {
	names, err := cm.LoadNamesFromFile()
	if err != nil {
		// Fallback names if file cannot be read
		names = []string{"client", "worker", "node", "service", "app"}
	}

	// Use fallback names if the loaded names array is empty
	if len(names) == 0 {
		names = []string{"client", "worker", "node", "service", "app"}
	}

	// Pick a random name
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(names))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}
	baseName := names[n.Int64()]

	// Add random suffix
	suffix, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}

	return fmt.Sprintf("%s-%d", baseName, suffix.Int64()), nil
}

// getContainerPorts extracts port mappings from a container
func (cm *ClientManager) getContainerPorts(ctx context.Context, containerID string) (string, string, error) {
	containerInfo, err := cm.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", "", err
	}

	// Look for port 9090 binding
	if bindings, exists := containerInfo.NetworkSettings.Ports["9090/tcp"]; exists && len(bindings) > 0 {
		hostPort := bindings[0].HostPort
		webURL := fmt.Sprintf("http://localhost:%s", hostPort)
		return hostPort, webURL, nil
	}

	return "", "", nil
}

// GetClientContainers returns a list of current client containers
func (cm *ClientManager) GetClientContainers(ctx context.Context) ([]ClientContainer, error) {
	containers, err := cm.dockerClient.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var clients []ClientContainer
	projectPrefix := fmt.Sprintf("%s-client-", cm.projectName)

	for _, cont := range containers {
		// Check if this is a client container by looking at container names
		for _, name := range cont.Names {
			// Remove leading slash from container name
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, projectPrefix) {
				// Get port information
				hostPort, webURL, err := cm.getContainerPorts(ctx, cont.ID)
				if err != nil {
					log.Printf("Warning: failed to get ports for container %s: %v", cleanName, err)
				}

				clients = append(clients, ClientContainer{
					Name:      cleanName,
					Status:    cont.Status,
					CreatedAt: fmt.Sprintf("%d", cont.Created),
					HostPort:  hostPort,
					WebURL:    webURL,
				})
				break
			}
		}
	}

	return clients, nil
}

// CreateClient creates a new client container with a custom name
func (cm *ClientManager) CreateClient(ctx context.Context, clientName string) error {
	if cm.project == nil {
		return fmt.Errorf("compose project not loaded")
	}

	clientService, exists := cm.project.Services["client"]
	if !exists {
		return fmt.Errorf("client service not found in compose project")
	}

	containerName := fmt.Sprintf("%s-client-%s", cm.projectName, clientName)

	// Determine the image to use
	var imageName string
	if clientService.Image != "" {
		imageName = clientService.Image
	} else if clientService.Build != nil {
		// For build context, we need to use the built image name
		// Docker Compose typically names built images as: project-service
		imageName = fmt.Sprintf("%s-client", cm.projectName)

		// Ensure the image exists or try to build it
		if err := cm.ensureImageExists(ctx, imageName, clientService); err != nil {
			return fmt.Errorf("failed to ensure image exists: %w", err)
		}
	} else {
		return fmt.Errorf("no image or build context found for client service")
	}

	// Prepare environment variables
	env := []string{
		fmt.Sprintf("CLIENT_NAME=%s", clientName),
		fmt.Sprintf("BOOTSTRAP_URL=http://bootstrap:8080"),
		"PORT=9090",
	}

	// Add any environment variables from the compose service, but skip ones we've already set
	skipVars := map[string]bool{
		"CLIENT_NAME":   true,
		"BOOTSTRAP_URL": true,
		"PORT":          true,
	}

	for key, value := range clientService.Environment {
		if value != nil && !skipVars[key] {
			env = append(env, fmt.Sprintf("%s=%s", key, *value))
		}
	}

	// Prepare command - use from service or default
	var cmd []string
	if len(clientService.Command) > 0 {
		cmd = clientService.Command
	}

	// Create container configuration
	config := &container.Config{
		Image: imageName,
		Env:   env,
		Labels: map[string]string{
			"com.docker.compose.project": cm.projectName,
			"com.docker.compose.service": "client",
		},
		ExposedPorts: nat.PortSet{
			"9090/tcp": struct{}{},
		},
	}

	// Only set Cmd if we have one from the service
	if len(cmd) > 0 {
		config.Cmd = cmd
	}

	// Host configuration with ephemeral port binding (Docker chooses the port)
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
		PortBindings: nat.PortMap{
			"9090/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "", // Empty string = ephemeral port (Docker chooses)
				},
			},
		},
	}

	// Network configuration - connect to the default network
	networkName := fmt.Sprintf("%s_default", cm.projectName)
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	// Create the container
	resp, err := cm.dockerClient.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start the container
	if err := cm.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// ensureImageExists checks if an image exists and provides helpful debugging info
func (cm *ClientManager) ensureImageExists(ctx context.Context, imageName string, service types.ServiceConfig) error {
	// Check if the image exists
	images, err := cm.dockerClient.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	imageFound := false
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == imageName || tag == imageName+":latest" {
				imageFound = true
				break
			}
		}
	}

	if !imageFound {
		// Suggest building the image
		if service.Build != nil && service.Build.Context != "" {
			buildPath := filepath.Join(cm.workingDir, "..", service.Build.Context)
			return fmt.Errorf("image %s not found. Please build it first using: docker build -t %s %s",
				imageName, imageName, buildPath)
		}

		return fmt.Errorf("image %s not found and no build context available", imageName)
	}

	return nil
}

// RemoveClient removes a specific client container
func (cm *ClientManager) RemoveClient(ctx context.Context, containerName string) error {
	// Stop the container with a timeout
	timeout := 10 // seconds
	err := cm.dockerClient.ContainerStop(ctx, containerName, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil && !client.IsErrNotFound(err) {
		log.Printf("Warning: failed to stop container %s: %v", containerName, err)
	}

	// Remove the container
	err = cm.dockerClient.ContainerRemove(ctx, containerName, container.RemoveOptions{
		Force: true,
	})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}

	return nil
}

// BulkCreateClients creates multiple client containers with random names
func (cm *ClientManager) BulkCreateClients(ctx context.Context, count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be positive")
	}

	var createdNames []string
	var errors []error

	for i := 0; i < count; i++ {
		name, err := cm.GenerateRandomName()
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to generate name for client %d: %w", i+1, err))
			continue
		}

		err = cm.CreateClient(ctx, name)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to create client %s: %w", name, err))
			continue
		}

		createdNames = append(createdNames, name)
	}

	return createdNames, nil
}
