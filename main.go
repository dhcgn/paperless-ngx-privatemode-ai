package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
	"github.com/dhcgn/paperless-ngx-privatemode-ai/processor"
	"github.com/pterm/pterm"
)

// Set in build time
var (
	Version   string = "dev"
	BuildTime string = "unknown"
	Commit    string = "unknown"
)

func main() {
	// Parse command line arguments
	configPath := flag.String("config", "", "Path to configuration file")
	autoSetTitles := flag.Bool("auto-set-titles-for-documents-from-pattern", false, "Automatically set titles for documents matching the configured pattern and exit")
	checkLLMConnection := flag.Bool("check-llm-connection", false, "Start the managed Privatemode proxy, verify the LLM service is accessible, then exit (skips Paperless NGX and action selection)")
	ocrFile := flag.String("ocr-file", "", "Perform OCR on a local image file using the LLM, print the extracted text, then exit (skips Paperless NGX and action selection)")
	flag.Parse()

	// Show banner
	showBanner()

	// Validate arguments
	if *configPath == "" {
		pterm.Error.Println("Configuration file is required. Use --config flag.")
		pterm.Info.Println("Usage: paperless-ngx-privatemode-ai.exe --config config.yaml")
		os.Exit(1)
	}

	// Initialize application
	app := &App{
		ConfigPath:         *configPath,
		AutoSetTitles:      *autoSetTitles,
		CheckLLMConnection: *checkLLMConnection,
		OCRFile:            *ocrFile,
	}

	// Run the application following the program flow
	if err := app.Run(); err != nil {
		pterm.Error.Printf("Application failed: %v\n", err)
		os.Exit(1)
	}
}

func showBanner() {
	pterm.DefaultCenter.Println(pterm.LightCyan("paperless-ngx-privatemode-ai"))
	pterm.DefaultCenter.Printf("Version: %s, Commit: %s, Build Time: %s\n", Version, Commit, BuildTime)
	pterm.DefaultCenter.Println(pterm.LightBlue("https://github.com/dhcgn/paperless-ngx-privatemode-ai"))

	pterm.Println()
	pterm.Warning.Println("⚠️  This project is in early development stage. It may not work as expected and may change in future releases.")
	pterm.Warning.Println("⚠️  Use it at your own risk, no warranty of any kind is provided.")
	pterm.Println()
}

type App struct {
	ConfigPath         string
	Config             *config.Config
	AutoSetTitles      bool
	CheckLLMConnection bool
	OCRFile            string
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Load configuration from argument --config
	pterm.Info.Println("Loading configuration...")
	config, err := config.LoadConfig(a.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	a.Config = config
	pterm.Success.Println("Configuration loaded successfully")

	// 2. Check configuration
	pterm.Info.Println("Validating configuration...")
	if err := a.Config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	pterm.Success.Println("Configuration is valid")

	skipPaperless := a.CheckLLMConnection || a.OCRFile != ""
	var paperlessClient *internal.PaperlessClient
	if !skipPaperless {
		// 3. Check if paperless-ngx is accessible
		pterm.Info.Println("Checking Paperless NGX accessibility...")
		paperlessClient = internal.NewPaperlessClient(a.Config)
		if err := paperlessClient.CheckConnection(); err != nil {
			return fmt.Errorf("paperless-ngx is not accessible: %w", err)
		}
		pterm.Success.Println("Paperless NGX is accessible")
	}

	// 4. Start the managed Privatemode proxy (replaces the previous manual docker run step)
	pterm.Info.Println("Starting managed Privatemode proxy...")
	managedProxy, err := internal.StartManagedProxy(ctx, a.Config)
	if err != nil {
		return fmt.Errorf("failed to start managed Privatemode proxy: %w", err)
	}
	defer managedProxy.Stop(10 * time.Second)
	pterm.Success.Println("Managed Privatemode proxy is running")

	// 5. Check if privatemode.ai is accessible and models are available
	pterm.Info.Println("Checking LLM service accessibility...")
	llmClient := internal.NewLLMClient(a.Config)
	if err := llmClient.CheckConnection(); err != nil {
		return fmt.Errorf("LLM service is not accessible: %w", err)
	}
	pterm.Success.Println("LLM service is accessible")

	if a.CheckLLMConnection {
		pterm.Success.Println("LLM connectivity check passed")
		return nil
	}

	if a.OCRFile != "" {
		return a.runOCRFile(llmClient)
	}

	// 6. Ask user for action
	action, autonomous, err := a.askUserForAction()
	if err != nil {
		return fmt.Errorf("failed to get user action: %w", err)
	}

	// 7. Execute action and show progress
	pterm.Info.Printf("Executing action: %s\n", action.Description())
	executor := processor.NewActionExecutor(paperlessClient, llmClient, a.Config, autonomous)
	return executor.Execute(action)
}

// runOCRFile reads a local image file, converts it to JPEG if needed, and prints the LLM's OCR output.
func (a *App) runOCRFile(llmClient *internal.LLMClient) error {
	pterm.Info.Printf("Reading file %s...\n", a.OCRFile)
	data, err := os.ReadFile(a.OCRFile)
	if err != nil {
		return fmt.Errorf("failed to read OCR input file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(a.OCRFile))
	if ext != ".jpg" && ext != ".jpeg" {
		pterm.Info.Printf("Converting %s to JPEG...\n", ext)
		data, err = internal.ConvertImageToJpg(a.Config, data, ext)
		if err != nil {
			return fmt.Errorf("failed to convert image to JPEG: %w", err)
		}
	}

	pterm.Info.Println("Extracting text via OCR...")
	text, err := llmClient.MakeOcr(data)
	if err != nil {
		return fmt.Errorf("failed to perform OCR: %w", err)
	}
	pterm.Success.Println("OCR extraction succeeded")

	fmt.Println("--- Extracted Content ---")
	fmt.Println(text)
	fmt.Println("--- End of Extracted Content ---")
	return nil
}

func (a *App) askUserForAction() (processor.Action, bool, error) {
	if a.AutoSetTitles {
		pterm.Info.Println("Automatically setting titles for documents matching the configured pattern...")
		return &processor.SetTitleAction{}, true, nil
	}

	pterm.Println()
	pterm.DefaultHeader.Println("Select an action:")
	pterm.Println()

	patternTitleJoined := strings.Join(a.Config.Filters.Title.Pattern, ", ")
	patternOcrJoined := strings.Join(a.Config.Filters.Content.Pattern, ", ")

	options := []string{
		fmt.Sprintf("Set titles from documents with pattern: '%s'", patternTitleJoined),
		fmt.Sprintf("Set content with OCR from documents with pattern: '%s'", patternOcrJoined),
		"Exit",
	}

	selectedOption, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultOption("Exit").
		Show("Choose an action:")

	if err != nil {
		return nil, false, fmt.Errorf("failed to get user selection: %w", err)
	}

	switch selectedOption {
	case options[0]:
		return &processor.SetTitleAction{}, false, nil
	case options[1]:
		return &processor.SetOcrInContentAction{}, false, nil
	case options[2]:
		pterm.Info.Println("Exiting...")
		os.Exit(0)
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("invalid selection")
	}
}
