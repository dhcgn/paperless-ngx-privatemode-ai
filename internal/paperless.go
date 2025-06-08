package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
	"github.com/pterm/pterm"
)

type PaperlessClient struct {
	config     *config.Config
	httpClient *http.Client
}

type Document struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	CreatedDate  string `json:"created_date"`
	DocumentType *int   `json:"document_type"`
}

type DocumentsResponse struct {
	Count   int        `json:"count"`
	Results []Document `json:"results"`
}

// FilterType represents the type of document filter to apply
type FilterType string

const (
	FilterTypeTitle   FilterType = "title"
	FilterTypeContent FilterType = "content"
)

func NewPaperlessClient(config *config.Config) *PaperlessClient {
	return &PaperlessClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *PaperlessClient) CheckConnection() error {
	url := strings.TrimSuffix(c.config.Paperless.API.BaseURL, "/") + "/api/documents/?page_size=1"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *PaperlessClient) GetDocuments() ([]Document, error) {
	url := strings.TrimSuffix(c.config.Paperless.API.BaseURL, "/") + "/api/documents/"
	if c.config.Paperless.API.PageSize > 0 {
		url += "?page_size=" + strconv.Itoa(c.config.Paperless.API.PageSize)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response DocumentsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Results, nil
}

// DownloadDocument
func (c *PaperlessClient) DownloadDocument(documentID int) ([]byte, error) {
	url := strings.TrimSuffix(c.config.Paperless.API.BaseURL, "/") + "/api/documents/" + strconv.Itoa(documentID) + "/download/"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func (c *PaperlessClient) UpdateDocument(documentID int, updates map[string]interface{}) error {
	url := strings.TrimSuffix(c.config.Paperless.API.BaseURL, "/") + "/api/documents/" + strconv.Itoa(documentID) + "/"

	updateData, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal update data: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, strings.NewReader(string(updateData)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *PaperlessClient) FilterDocuments(documents []Document, filterType FilterType) ([]Document, error) {
	var filtered []Document
	var patterns []string

	switch filterType {
	case FilterTypeTitle:
		patterns = c.config.Filters.Title.Pattern
	case FilterTypeContent:
		patterns = c.config.Filters.Content.Pattern
	default:
		return nil, fmt.Errorf("unknown filter type: %s", filterType)
	}

	progressBar, _ := pterm.DefaultProgressbar.
		WithTitle("Filtering documents").
		WithTotal(len(documents)).
		Start()

	for _, doc := range documents {
		var targetText string
		switch filterType {
		case FilterTypeTitle:
			targetText = doc.Title
		case FilterTypeContent:
			targetText = doc.Content
		}

		matched := false
		for _, pattern := range patterns {
			regex, err := regexp.Compile(pattern)
			if err != nil {
				progressBar.Stop()
				return nil, fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
			}

			if regex.MatchString(targetText) {
				matched = true
				break
			}
		}

		if matched {
			filtered = append(filtered, doc)
		}

		progressBar.Increment()
	}

	progressBar.Stop()
	return filtered, nil
}

func (c *PaperlessClient) addHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Token "+c.config.Paperless.API.Token)
	if c.config.Paperless.API.HostHeader != "" {
		req.Header.Set("Host", c.config.Paperless.API.HostHeader)
		// Set Host Header directly in map
		req.Host = c.config.Paperless.API.HostHeader
		req.Header["Host"] = []string{c.config.Paperless.API.HostHeader}
	}
}
