package main

import (
	"testing"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
)

func TestConfigValidation(t *testing.T) {
	config := &config.Config{
		Paperless: config.PaperlessConfig{},
		LLM:       config.LLMConfig{},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error for empty config")
	}

	// Test with valid config
	config.Paperless.API.BaseURL = "http://localhost:8000"
	config.Paperless.API.Token = "test-token"
	config.LLM.API.BaseURL = "http://localhost:9876"
	config.LLM.Models.TitleGeneration = "test-model"
	config.LLM.Models.OCR = "test-model"

	err = config.Validate()
	if err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
}

func TestFilterConfig(t *testing.T) {
	config := &config.Config{
		Filters: config.FiltersConfig{
			Title: config.FilterConfig{
				PatternType: "regex",
				Pattern:     []string{"^SCN_.*$", "[invalid"},
			},
		},
	}

	// Set required fields to pass other validations
	config.Paperless.API.BaseURL = "http://localhost:8000"
	config.Paperless.API.Token = "test-token"
	config.LLM.API.BaseURL = "http://localhost:9876"
	config.LLM.Models.TitleGeneration = "test-model"
	config.LLM.Models.OCR = "test-model"

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error for invalid regex pattern")
	}

	// Fix the regex pattern
	config.Filters.Title.Pattern = []string{"^SCN_.*$", ".*BRN.*$"}
	err = config.Validate()
	if err != nil {
		t.Errorf("Expected no validation error, got: %v", err)
	}
}
