package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mini-k8s-orchestration/pkg/types"
)

// APIClient is the interface for communicating with the API server
type APIClient interface {
	RegisterNode(node *types.Node) error
	UpdateNodeStatus(nodeName string, status *types.NodeStatus) error
	GetAssignedPods(nodeName string) ([]*types.Pod, error)
	UpdatePodStatus(pod *types.Pod, status *types.PodStatus) error
}

// HTTPAPIClient implements APIClient using HTTP
type HTTPAPIClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL string) APIClient {
	return &HTTPAPIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterNode registers a node with the API server
func (c *HTTPAPIClient) RegisterNode(node *types.Node) error {
	url := fmt.Sprintf("%s/api/v1/nodes", c.baseURL)
	return c.postJSON(url, node)
}

// UpdateNodeStatus updates the status of a node
func (c *HTTPAPIClient) UpdateNodeStatus(nodeName string, status *types.NodeStatus) error {
	url := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", c.baseURL, nodeName)
	return c.postJSON(url, status)
}

// GetAssignedPods gets the pods assigned to a node
func (c *HTTPAPIClient) GetAssignedPods(nodeName string) ([]*types.Pod, error) {
	url := fmt.Sprintf("%s/api/v1/nodes/%s/pods", c.baseURL, nodeName)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned pods: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get assigned pods: status code %d", resp.StatusCode)
	}
	
	var podList struct {
		Items []*types.Pod `json:"items"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return nil, fmt.Errorf("failed to decode pod list: %w", err)
	}
	
	return podList.Items, nil
}

// UpdatePodStatus updates the status of a pod
func (c *HTTPAPIClient) UpdatePodStatus(pod *types.Pod, status *types.PodStatus) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/status", 
		c.baseURL, pod.Metadata.Namespace, pod.Metadata.Name)
	
	return c.putJSON(url, status)
}

// postJSON sends a POST request with JSON body
func (c *HTTPAPIClient) postJSON(url string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}

// putJSON sends a PUT request with JSON body
func (c *HTTPAPIClient) putJSON(url string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PUT request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}