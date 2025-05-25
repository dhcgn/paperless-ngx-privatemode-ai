package main

import (
	"flag"
	"fmt"
	"os"

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
		ConfigPath: *configPath,
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
	ConfigPath string
	Config     *Config
}

func (a *App) Run() error {
	// 1. Load configuration from argument --config
	pterm.Info.Println("Loading configuration...")
	config, err := LoadConfig(a.ConfigPath)
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
	paperlessClient := NewPaperlessClient(a.Config)
	if err := paperlessClient.CheckConnection(); err != nil {
		return fmt.Errorf("paperless-ngx is not accessible: %w", err)
	}
	pterm.Success.Println("Paperless NGX is accessible")

	// 4. Check if privatemode.ai is accessible and models are available
	pterm.Info.Println("Checking LLM service accessibility...")
	llmClient := NewLLMClient(a.Config)
	if err := llmClient.CheckConnection(); err != nil {
		return fmt.Errorf("LLM service is not accessible: %w", err)
	}
	pterm.Success.Println("LLM service is accessible")

	// 5. Ask user for action
	action, err := a.askUserForAction()
	if err != nil {
		return fmt.Errorf("failed to get user action: %w", err)
	}

	// 6. Execute action and show progress
	pterm.Info.Printf("Executing action: %s\n", action.Description())
	executor := NewActionExecutor(paperlessClient, llmClient, a.Config)
	return executor.Execute(action)
}

func (a *App) askUserForAction() (Action, error) {
	pterm.Println()
	pterm.DefaultHeader.Println("Select an action:")
	pterm.Println()

	options := []string{
		"Set document titles which title contains pattern",
		"Set document content which content contains pattern",
		"Set document content and title which contains pattern",
		"Set document content and title which contains LLM response contains pattern",
		"Exit",
	}

	selectedOption, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultOption("Exit").
		Show("Choose an action:")

	if err != nil {
		return nil, fmt.Errorf("failed to get user selection: %w", err)
	}

	switch selectedOption {
	case options[0]:
		return &SetTitleAction{}, nil
	case options[1]:
		return &SetContentAction{}, nil
	case options[2]:
		return &SetTitleAndContentAction{}, nil
	case options[3]:
		return &SetTitleAndContentWithLLMAction{}, nil
	case options[4]:
		pterm.Info.Println("Exiting...")
		os.Exit(0)
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid selection")
	}
}
