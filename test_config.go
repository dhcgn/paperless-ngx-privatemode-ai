package main

import (
	"fmt"
	"log"
	"runtime"
)

func testMain() {
	config, err := LoadConfig("config.sample.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Configuration loaded successfully!\n")
	fmt.Printf("OS detection: Windows = %t\n", config.isWindows())

	// Test ImageMagick validation
	if err := config.validateImageMagick(); err != nil {
		fmt.Printf("ImageMagick validation failed: %v\n", err)
	} else {
		fmt.Printf("ImageMagick validation successful!\n")
	}

	// Test getting ImageMagick command
	cmd, err := config.getImageMagickCommand()
	if err != nil {
		fmt.Printf("Failed to get ImageMagick command: %v\n", err)
	} else {
		fmt.Printf("ImageMagick command: %s\n", cmd)
	}
}

func (c *Config) isWindows() bool {
	return runtime.GOOS == "windows"
}
