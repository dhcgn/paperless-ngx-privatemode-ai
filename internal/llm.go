package internal

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
)

//go:embed llm_assets/schema_title_generation.json
var schema_title_generation []byte

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
	config     *config.Config
	httpClient *http.Client
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []ChatMessage   `json:"messages"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

type ResponseFormat struct {
	Type       string     `json:"type"`
	JSONSchema JSONSchema `json:"json_schema"`
}

type JSONSchema struct {
	Name   string      `json:"name"`
	Schema interface{} `json:"schema"`
	Strict bool        `json:"strict"`
}

type CaptionResponse struct {
	Summarize string    `json:"summarize"`
	Captions  []Caption `json:"captions"`
}

type Caption struct {
	Caption string  `json:"caption"`
	Score   float64 `json:"score"`
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

func NewLLMClient(config *config.Config) *LLMClient {
	// Use configured timeout or default to 300 seconds (5 minutes)
	timeout := config.LLM.API.Timeout
	if timeout <= 0 {
		timeout = 300 // Default to 5 minutes
	}

	return &LLMClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
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

	allRemoteModelsIds := []string{}

	for _, model := range modelsResp.Data {
		allRemoteModelsIds = append(allRemoteModelsIds, model.ID)
		if model.ID == c.config.LLM.Models.TitleGeneration {
			titleModelAvailable = true
		}
		if model.ID == c.config.LLM.Models.OCR {
			contentModelAvailable = true
		}
	}

	if !titleModelAvailable {
		return fmt.Errorf("title generation model '%s' not available, found %v", c.config.LLM.Models.TitleGeneration, allRemoteModelsIds)
	}
	if !contentModelAvailable {

		return fmt.Errorf("content extraction model '%s' not available, found %v", c.config.LLM.Models.OCR, allRemoteModelsIds)
	}

	return nil
}

func (c *LLMClient) GenerateTitleFromContent(content string) (CaptionResponse, error) {
	if content == "" {
		return CaptionResponse{
			Summarize: "Empty document content",
			Captions: []Caption{
				{Caption: "EMPTY_CONTENT", Score: 0.0},
			},
		}, nil
	}

	// Truncate content if configured
	if c.config.Processing.TitleGeneration.TruncateCharactersOfContent > 0 &&
		len(content) > c.config.Processing.TitleGeneration.TruncateCharactersOfContent {
		content = content[:c.config.Processing.TitleGeneration.TruncateCharactersOfContent]
	}

	// Create the structured prompt for title generation
	prompt := fmt.Sprintf(`You are an AI designed to generate descriptive titles for document content.
Please provide a list of possible titles, each with a relevance score between 0 and 1 that indicates how well the title describes the content.
Your response must be a JSON object matching the following schema:

- Include at least 3 titles.
- Do not include any extra information or explanation.
- Keep the language of the original document.

Task:
Given the content, return your output in the exact JSON structure as described above.

Content:

%s`, content)

	response, err := c.sendStructuredChatRequest(c.config.LLM.Models.TitleGeneration, prompt)
	if err != nil {
		return CaptionResponse{}, fmt.Errorf("failed to generate title: %w", err)
	}

	// Parse the structured response
	var captionResp CaptionResponse
	if err := json.Unmarshal([]byte(response), &captionResp); err != nil {
		// If JSON parsing fails, return the raw response as a single caption
		return CaptionResponse{
			Summarize: "Failed to parse LLM response",
			Captions:  []Caption{{Caption: response, Score: 0.0}},
		}, nil
	}

	if len(captionResp.Captions) == 0 {
		return CaptionResponse{
			Summarize: captionResp.Summarize,
			Captions:  []Caption{{Caption: response, Score: 0.0}},
		}, nil
	}

	return captionResp, nil
}

func (c *LLMClient) MakeOcr(imageData []byte) (string, error) {
	// check if image data is jpg
	if len(imageData) < 2 || (imageData[0] != 0xFF || imageData[1] != 0xD8) {
		return "", fmt.Errorf("invalid image data: not a valid JPEG")
	}

	response, err := c.sendOCRRequest(c.config.LLM.Models.OCR, c.config.LLM.Prompts.OCR, imageData)
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

	if c.config.LLM.API.Debug && c.config.LLM.API.DebugFolder != "" {
		err = saveDebugScript(url, reqBody, c.config.LLM.API.DebugFolder, "ocr-request")
		if err != nil {
			return "", fmt.Errorf("failed to save debug script: %w", err)
		} else {
			fmt.Printf("Debug script saved to %s\n", c.config.LLM.API.DebugFolder)
		}
	}

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

func saveDebugScript(url string, reqBody []byte, debugFolder string, name string) error {
	script := fmt.Sprintf(`#!/bin/bash
curl -X POST %s \
-H "Content-Type: application/json" \
-d '%s'
	`, url, reqBody)
	timestamp := time.Now().Format("20060102-150405")
	filepath := filepath.Join(debugFolder, timestamp+"-"+name+".sh")
	err := os.WriteFile(filepath, []byte(script), 0644)
	if err != nil {
		return fmt.Errorf("failed to write debug script: %w", err)
	}
	return nil
}

// func (c *LLMClient) sendChatRequest(model, prompt string) (string, error) {
// 	url := strings.TrimSuffix(c.config.LLM.API.BaseURL, "/") + c.config.LLM.API.Endpoint

// 	chatReq := ChatRequest{
// 		Model: model,
// 		Messages: []ChatMessage{
// 			{
// 				Role:    "user",
// 				Content: prompt,
// 			},
// 		},
// 	}

// 	reqBody, err := json.Marshal(chatReq)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to marshal request: %w", err)
// 	}

// 	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create request: %w", err)
// 	}

// 	req.Header.Set("Content-Type", "application/json")

// 	if c.config.LLM.API.Debug && c.config.LLM.API.DebugFolder != "" {
// 		err = saveDebugScript(url, reqBody, c.config.LLM.API.DebugFolder, "title-request")
// 		if err != nil {
// 			return "", fmt.Errorf("failed to save debug script: %w", err)
// 		} else {
// 			fmt.Printf("Debug script saved to %s\n", c.config.LLM.API.DebugFolder)
// 		}
// 	}

// 	resp, err := c.httpClient.Do(req)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to send request: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return "", fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
// 	}

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read response: %w", err)
// 	}

// 	var chatResp ChatResponse
// 	if err := json.Unmarshal(body, &chatResp); err != nil {
// 		return "", fmt.Errorf("failed to parse response: %w", err)
// 	}

// 	if len(chatResp.Choices) == 0 {
// 		return "", fmt.Errorf("no choices in response")
// 	}

// 	return chatResp.Choices[0].Message.Content, nil
// }

func (c *LLMClient) sendStructuredChatRequest(model, prompt string) (string, error) {
	url := strings.TrimSuffix(c.config.LLM.API.BaseURL, "/") + c.config.LLM.API.Endpoint

	var schema interface{}
	if err := json.Unmarshal(schema_title_generation, &schema); err != nil {
		return "", fmt.Errorf("failed to parse schema: %w", err)
	}

	// Extract the schema content from the parsed JSON
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid schema format")
	}

	chatReq := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		ResponseFormat: &ResponseFormat{
			Type: "json_schema",
			JSONSchema: JSONSchema{
				Name:   schemaMap["name"].(string),
				Schema: schemaMap["schema"],
				Strict: schemaMap["strict"].(bool),
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

	if c.config.LLM.API.Debug && c.config.LLM.API.DebugFolder != "" {
		err = saveDebugScript(url, reqBody, c.config.LLM.API.DebugFolder, "title-request")
		if err != nil {
			return "", fmt.Errorf("failed to save debug script: %w", err)
		} else {
			fmt.Printf("Debug script saved to %s\n", c.config.LLM.API.DebugFolder)
		}
	}

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
