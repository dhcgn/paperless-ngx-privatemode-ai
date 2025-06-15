package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
		ConfigPath:    *configPath,
		AutoSetTitles: *autoSetTitles,
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
	ConfigPath    string
	Config        *config.Config
	AutoSetTitles bool
}

func (a *App) Run() error {
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

	// 3. Check if paperless-ngx is accessible
	pterm.Info.Println("Checking Paperless NGX accessibility...")
	paperlessClient := internal.NewPaperlessClient(a.Config)
	if err := paperlessClient.CheckConnection(); err != nil {
		return fmt.Errorf("paperless-ngx is not accessible: %w", err)
	}
	pterm.Success.Println("Paperless NGX is accessible")

	// 4. Check if privatemode.ai is accessible and models are available
	pterm.Info.Println("Checking LLM service accessibility...")
	llmClient := internal.NewLLMClient(a.Config)
	if err := llmClient.CheckConnection(); err != nil {
		return fmt.Errorf("LLM service is not accessible: %w", err)
	}
	pterm.Success.Println("LLM service is accessible")

	// 5. Ask user for action
	action, autonomous, err := a.askUserForAction()
	if err != nil {
		return fmt.Errorf("failed to get user action: %w", err)
	}

	// 6. Execute action and show progress
	pterm.Info.Printf("Executing action: %s\n", action.Description())
	executor := processor.NewActionExecutor(paperlessClient, llmClient, a.Config, autonomous)
	return executor.Execute(action)
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
