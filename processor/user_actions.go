package processor

import (
	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
	"github.com/pterm/pterm"
)

type UserOption string

const (
	Undefined                UserOption = ""
	skipDocumentOption       UserOption = "Skip this document"
	makeOcrAndTryAgainOption UserOption = "Make OCR and try again"
	customTitleOption        UserOption = "Enter custom title"
)

type Action interface {
	Description() string
	Execute(executor *ActionExecutor) error
}

type ActionExecutor struct {
	paperlessClient *internal.PaperlessClient
	llmClient       *internal.LLMClient
	config          *config.Config
}

func NewActionExecutor(paperlessClient *internal.PaperlessClient, llmClient *internal.LLMClient, config *config.Config) *ActionExecutor {
	return &ActionExecutor{
		paperlessClient: paperlessClient,
		llmClient:       llmClient,
		config:          config,
	}
}

func (e *ActionExecutor) Execute(action Action) error {
	return action.Execute(e)
}

type ProgressStats struct {
	total     int
	processed int
	success   int
	errors    int
	skipped   int
}

func (stats *ProgressStats) renderProgressChart() {
	bars := []pterm.Bar{
		{Label: "Total", Value: stats.total, Style: pterm.NewStyle(pterm.FgGray)},
		{Label: "Processed", Value: stats.processed, Style: pterm.NewStyle(pterm.FgBlue)},
		{Label: "Success", Value: stats.success, Style: pterm.NewStyle(pterm.FgGreen)},
		{Label: "Errors", Value: stats.errors, Style: pterm.NewStyle(pterm.FgRed)},
		{Label: "Skipped", Value: stats.skipped, Style: pterm.NewStyle(pterm.FgYellow)},
	}
	pterm.DefaultBarChart.WithHorizontal().WithBars(bars).WithShowValue().Render()
}

func (stats *ProgressStats) renderFinalSummary(totalDocuments int) {
	pterm.Success.Printf("Successfully updated %d documents\n", stats.success)
	if stats.errors > 0 {
		pterm.Warning.Printf("Failed to update %d documents\n", stats.errors)
	}
	if stats.skipped > 0 {
		pterm.Info.Printf("Skipped %d documents\n", stats.skipped)
	}

	bars := []pterm.Bar{
		{Label: "Total", Value: totalDocuments, Style: pterm.NewStyle(pterm.FgGray)},
		{Label: "Success", Value: stats.success, Style: pterm.NewStyle(pterm.FgGreen)},
		{Label: "Errors", Value: stats.errors, Style: pterm.NewStyle(pterm.FgRed)},
		{Label: "Skipped", Value: stats.skipped, Style: pterm.NewStyle(pterm.FgYellow)},
	}
	pterm.Info.Println("Final Summary:")
	pterm.DefaultBarChart.WithHorizontal().WithBars(bars).WithShowValue().Render()
}
