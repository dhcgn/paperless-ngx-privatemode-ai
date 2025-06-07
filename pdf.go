package main

// renderPageToJpg converts a specific page of a PDF document to a JPEG image.
// It takes the PDF bytes and the page number as input, and returns the JPEG image bytes or an error.
import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// getImageMagickCommand returns the appropriate ImageMagick command based on the OS
func (c *Config) getImageMagickCommand() (string, error) {
	if runtime.GOOS == "windows" {
		// On Windows, use the configured path for imagemagick-for-windows
		if c.Tools.ImagemagickForWindows.FullPath != "" {
			return c.Tools.ImagemagickForWindows.FullPath, nil
		}
		return "", fmt.Errorf("imagemagick-for-windows.fullpath not set in config for Windows")
	} else {
		// On Linux/Unix, check for system-installed ImageMagick
		if magickPath, err := exec.LookPath("magick"); err == nil {
			return magickPath, nil
		}
		// Fallback to older ImageMagick command name
		if convertPath, err := exec.LookPath("convert"); err == nil {
			return convertPath, nil
		}
		return "", fmt.Errorf("ImageMagick not found in system PATH. Please install ImageMagick")
	}
}

// RenderPageToJpg converts a specific page of a PDF document to a JPEG image using ImageMagick.
func (c *Config) RenderPageToJpg(pdfBytes []byte, page int) ([]byte, error) {
	// 1. Write PDF bytes to a temporary file
	pdfFile, err := os.CreateTemp("", "input-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	defer os.Remove(pdfFile.Name())

	if _, err := pdfFile.Write(pdfBytes); err != nil {
		pdfFile.Close()
		return nil, fmt.Errorf("failed to write PDF bytes: %w", err)
	}
	pdfFile.Close()

	// 2. Prepare output JPG temp file
	jpgFile, err := os.CreateTemp("", "output-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp JPG file: %w", err)
	}
	jpgFilePath := jpgFile.Name()
	jpgFile.Close()
	defer os.Remove(jpgFilePath)

	// 3. Build ImageMagick command
	// ImageMagick uses 0-based page index: input.pdf[0] for first page
	pdfInputWithPage := fmt.Sprintf("%s[%d]", pdfFile.Name(), page)
	magickPath, err := c.getImageMagickCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to get ImageMagick command: %w", err)
	}

	cmd := exec.Command(magickPath, pdfInputWithPage, jpgFilePath)

	// 4. Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ImageMagick failed: %w, output: %s", err, string(output))
	}

	// 5. Read the resulting JPG file
	jpgBytes, err := os.ReadFile(jpgFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output JPG: %w", err)
	}

	return jpgBytes, nil
}
