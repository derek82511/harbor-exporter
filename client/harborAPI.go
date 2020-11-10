package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-resty/resty/v2"
)

// Repository Definition
type Repository struct {
	Name string `json:"name"`
}

// Artifact Definition
type Artifact struct {
	Tags []Tag `json:"tags"`
}

// Tag Definition
type Tag struct {
	Name string `json:"name"`
}

// Harbor API Constant
const (
	APISchema            = "https"
	APIUrlListRepository = "/api/v2.0/projects/%s/repositories"
	APIUrlListArtifact   = "/api/v2.0/projects/%s/repositories/%s/artifacts"
)

// GetRepositories Function
func GetRepositories(client *resty.Client, registry string, project string) *[]Repository {
	url := fmt.Sprintf("%s://%s%s?&page_size=-1", APISchema, registry, fmt.Sprintf(APIUrlListRepository, project))

	resp, err := client.R().Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if resp.StatusCode() != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: %v\n", resp)
		os.Exit(1)
	}

	repositories := &[]Repository{}

	json.Unmarshal(resp.Body(), &repositories)

	return repositories
}

// GetArtifacts Function
func GetArtifacts(client *resty.Client, registry string, project string, repository string) *[]Artifact {
	url := fmt.Sprintf("%s://%s%s?&page_size=-1&with_tag=true&with_label=false&with_scan_overview=false&with_signature=false&with_immutable_status=false", APISchema, registry, fmt.Sprintf(APIUrlListArtifact, project, repository))

	resp, err := client.R().Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if resp.StatusCode() != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: %v\n", resp)
		os.Exit(1)
	}

	artifacts := &[]Artifact{}

	json.Unmarshal(resp.Body(), &artifacts)

	return artifacts
}
