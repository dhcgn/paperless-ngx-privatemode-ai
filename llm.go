package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Vision types for OCR chat request
type MessageContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type VisionChatMessage struct {
	Role    string           `json:"role"`
	Content []MessageContent `json:"content"`
}

type VisionChatRequest struct {
	Model    string              `json:"model"`
	Messages []VisionChatMessage `json:"messages"`
}

type LLMClient struct {
	config     *Config
	httpClient *http.Client
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

type ModelInfo struct {
	ID string `json:"id"`
}

type ModelsResponse struct {
	Data []ModelInfo `json:"data"`
}

func NewLLMClient(config *Config) *LLMClient {
	return &LLMClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *LLMClient) CheckConnection() error {
	// Check models endpoint
	url := strings.TrimSuffix(c.config.LLM.API.BaseURL, "/") + "/v1/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse and check if our required models are available
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var modelsResp ModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return fmt.Errorf("failed to parse models response: %w", err)
	}

	// Check if required models are available
	titleModelAvailable := false
	contentModelAvailable := false

	for _, model := range modelsResp.Data {
		if model.ID == c.config.LLM.Models.TitleGeneration {
			titleModelAvailable = true
		}
		if model.ID == c.config.LLM.Models.ContentExtraction {
			contentModelAvailable = true
		}
	}

	if !titleModelAvailable {
		return fmt.Errorf("title generation model '%s' not available", c.config.LLM.Models.TitleGeneration)
	}
	if !contentModelAvailable {
		return fmt.Errorf("content extraction model '%s' not available", c.config.LLM.Models.ContentExtraction)
	}

	return nil
}

func (c *LLMClient) GenerateTitleFromContent(content string) ([]string, error) {
	if content == "" {
		return []string{
			"EMPTY_CONTENT",
		}, nil
	}

	// Validate content length	// Truncate content if configured
	if c.config.Processing.TitleGeneration.TruncateCharactersOfContent > 0 &&
		len(content) > c.config.Processing.TitleGeneration.TruncateCharactersOfContent {
		content = content[:c.config.Processing.TitleGeneration.TruncateCharactersOfContent]
	}

	// Replace placeholder in prompt
	prompt := strings.ReplaceAll(c.config.LLM.Prompts.TitleGeneration, "{content}", content)
	prompt = strings.ReplaceAll(prompt, "{truncate_chars}",
		fmt.Sprintf("%d", c.config.Processing.TitleGeneration.TruncateCharactersOfContent))

	response, err := c.sendChatRequest(c.config.LLM.Models.TitleGeneration, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate title: %w", err)
	}

	// Parse JSON response to extract titles
	var titles []string
	if err := json.Unmarshal([]byte(response), &titles); err != nil {
		// If JSON parsing fails, return the raw response as a single title
		return []string{response}, nil
	}

	return titles, nil
}

func (c *LLMClient) ExtractContent(imageData []byte) (string, error) {
	// For now, this is a placeholder as image processing requires base64 encoding
	// and specific message format for vision models
	response, err := c.sendOCRRequest(c.config.LLM.Models.ContentExtraction, c.config.LLM.Prompts.ContentExtraction, imageData)
	if err != nil {
		return "", fmt.Errorf("failed to extract content: %w", err)
	}

	return response, nil
}

func (c *LLMClient) sendOCRRequest(model, prompt string, imageData []byte) (string, error) {
	url := strings.TrimSuffix(c.config.LLM.API.BaseURL, "/") + c.config.LLM.API.Endpoint

	// Prepare base64 image and data URL (no mime type, as in your example)
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:;base64," + base64Image

	chatReq := VisionChatRequest{
		Model: model,
		Messages: []VisionChatMessage{
			{
				Role: "user",
				Content: []MessageContent{
					{
						Type: "text",
						Text: prompt,
					},
					{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL: dataURL,
						},
					},
				},
			},
		},
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (c *LLMClient) sendChatRequest(model, prompt string) (string, error) {
	url := strings.TrimSuffix(c.config.LLM.API.BaseURL, "/") + c.config.LLM.API.Endpoint

	chatReq := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}
