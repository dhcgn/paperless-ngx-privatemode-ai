package config

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Paperless  PaperlessConfig  `yaml:"paperless"`
	LLM        LLMConfig        `yaml:"llm"`
	Filters    FiltersConfig    `yaml:"filters"`
	Processing ProcessingConfig `yaml:"processing"`
	Tools      ToolsConfig      `yaml:"tools"`
}
type ToolsConfig struct {
	ImagemagickForWindows ImagemagickConfig `yaml:"imagemagick-for-windows"`
}

type ImagemagickConfig struct {
	FullPath string `yaml:"fullpath"`
}

type PaperlessConfig struct {
	API struct {
		BaseURL    string `yaml:"base_url"`
		HostHeader string `yaml:"host_header"`
		Token      string `yaml:"token"`
		PageSize   int    `yaml:"page_size"`
	} `yaml:"api"`
	WebURL string `yaml:"web_url"`
}

type LLMConfig struct {
	API struct {
		BaseURL  string `yaml:"base_url"`
		Endpoint string `yaml:"endpoint"`
		Timeout  int    `yaml:"timeout"` // Timeout in seconds for LLM API requests
	} `yaml:"api"`
	Models struct {
		TitleGeneration string `yaml:"title_generation"`
		OCR             string `yaml:"ocr"`
	} `yaml:"models"`
	Prompts struct {
		TitleGeneration string `yaml:"title_generation"`
		OCR             string `yaml:"ocr"`
	} `yaml:"prompts"`
}

type FiltersConfig struct {
	Title   FilterConfig `yaml:"title"`
	Content FilterConfig `yaml:"content"`
}

type FilterConfig struct {
	PatternType string   `yaml:"pattern_type"`
	Pattern     []string `yaml:"pattern"`
}

type ProcessingConfig struct {
	TitleGeneration struct {
		TruncateCharactersOfContent int `yaml:"truncate_characters_of_content"`
	} `yaml:"title_generation"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	// Validate Paperless NGX config
	if c.Paperless.API.BaseURL == "" {
		return fmt.Errorf("paperless.api.base_url is required")
	}
	if c.Paperless.API.Token == "" {
		return fmt.Errorf("paperless.api.token is required")
	}

	// Validate LLM config
	if c.LLM.API.BaseURL == "" {
		return fmt.Errorf("llm.api.base_url is required")
	}
	if c.LLM.Models.TitleGeneration == "" {
		return fmt.Errorf("llm.models.title_generation is required")
	}
	if c.LLM.Models.OCR == "" {
		return fmt.Errorf("llm.models.ocr is required")
	}

	// Validate regex patterns
	for _, pattern := range c.Filters.Title.Pattern {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid title filter pattern '%s': %w", pattern, err)
		}
	}
	for _, pattern := range c.Filters.Content.Pattern {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid content filter pattern '%s': %w", pattern, err)
		}
	}

	// Check for Prompts
	if c.LLM.Prompts.TitleGeneration == "" {
		return fmt.Errorf("llm.prompts.title_generation is required")
	}
	if c.LLM.Prompts.OCR == "" {
		return fmt.Errorf("llm.prompts.ocr is required")
	}

	// Validate ImageMagick availability
	if err := c.validateImageMagick(); err != nil {
		return fmt.Errorf("imagemagick validation failed: %w", err)
	}

	return nil
}

// validateImageMagick validates ImageMagick availability based on the operating system
func (c *Config) validateImageMagick() error {
	if runtime.GOOS == "windows" {
		// On Windows, validate the specific path for imagemagick-for-windows
		if c.Tools.ImagemagickForWindows.FullPath != "" {
			if _, err := os.Stat(c.Tools.ImagemagickForWindows.FullPath); err != nil {
				return fmt.Errorf("imagemagick-for-windows.fullpath does not exist or is not accessible: %w", err)
			}
		}
	} else {
		// On Linux/Unix, check if ImageMagick is installed in the system
		_, err := exec.LookPath("magick")
		if err != nil {
			// Fallback to older ImageMagick command name
			_, err = exec.LookPath("convert")
			if err != nil {
				return fmt.Errorf("ImageMagick not found in system PATH. Please install ImageMagick")
			}
		}
	}
	return nil
}

func (c *Config) CreateUrl(docID int) string {
	if c.Paperless.WebURL == "" {
		return fmt.Sprintf("%s/documents/%d", c.Paperless.API.BaseURL, docID)
	}
	return fmt.Sprintf("%s/documents/%d", c.Paperless.WebURL, docID)
}
