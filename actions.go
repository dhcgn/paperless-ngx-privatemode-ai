package main

import (
	"fmt"
	"sort"

	"github.com/pterm/pterm"
)

type Action interface {
	Description() string
	Execute(executor *ActionExecutor) error
}

type ActionExecutor struct {
	paperlessClient *PaperlessClient
	llmClient       *LLMClient
	config          *Config
}

func NewActionExecutor(paperlessClient *PaperlessClient, llmClient *LLMClient, config *Config) *ActionExecutor {
	return &ActionExecutor{
		paperlessClient: paperlessClient,
		llmClient:       llmClient,
		config:          config,
	}
}

func (e *ActionExecutor) Execute(action Action) error {
	return action.Execute(e)
}

// SetTitleAction - Set document titles which title contains pattern
type SetTitleAction struct{}

func (a *SetTitleAction) Description() string {
	return "Set document titles which title contains pattern"
}

func (a *SetTitleAction) Execute(executor *ActionExecutor) error {
	// Get all documents
	pterm.Info.Println("Fetching documents from Paperless NGX...")
	documents, err := executor.paperlessClient.GetDocuments()
	if err != nil {
		return fmt.Errorf("failed to get documents: %w", err)
	}
	pterm.Success.Printf("Fetched %d documents\n", len(documents))

	// Filter documents by title pattern
	pterm.Info.Println("Filtering documents by title pattern...")
	filteredDocs, err := executor.paperlessClient.FilterDocuments(documents, "title")
	if err != nil {
		return fmt.Errorf("failed to filter documents: %w", err)
	}
	pterm.Success.Printf("Found %d documents matching title patterns\n", len(filteredDocs))

	if len(filteredDocs) == 0 {
		pterm.Warning.Println("No documents found matching the title patterns")
		return nil
	}

	// Display bar chart with document counts
	bars := []pterm.Bar{
		{Label: "All", Value: len(documents), Style: pterm.NewStyle(pterm.FgGray)},
		{Label: "Found", Value: len(filteredDocs), Style: pterm.NewStyle(pterm.FgGreen)},
	}
	pterm.DefaultBarChart.WithHorizontal().WithBars(bars).WithShowValue().Render()

	// Ask for confirmation
	confirmed, err := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(false).
		WithDefaultText("Do you want to generate new titles for these documents using LLM?").
		Show()
	if err != nil {
		return fmt.Errorf("failed to get confirmation: %w", err)
	}

	if !confirmed {
		pterm.Info.Println("Operation cancelled by user")
		return nil
	}

	// Process documents
	return executor.processDocumentsForTitleGeneration(filteredDocs, func(doc Document, captionResp CaptionResponse) (string, bool) {
		// Show document summary first
		if captionResp.Summarize != "" {
			pterm.Info.Printf("Document Summary: %s\n\n", captionResp.Summarize)
		}

		// Sort captions by score (highest score first)
		sort.Slice(captionResp.Captions, func(i, j int) bool {
			return captionResp.Captions[i].Score > captionResp.Captions[j].Score
		})

		// Prepare options for user selection
		options := make([]string, 0, len(captionResp.Captions)+1)

		// Add each caption with its score
		for i, caption := range captionResp.Captions {
			options = append(options, fmt.Sprintf("%d. %s (Score: %.2f)", i+1, caption.Caption, caption.Score))
		}

		// Add option to skip
		options = append(options, "Skip this document")

		// Show interactive select
		selectedOption, err := pterm.DefaultInteractiveSelect.
			WithOptions(options).
			WithDefaultOption("Skip this document").
			Show(fmt.Sprintf("Choose a new title for document '%s':\nUrl: %s\n", doc.Title, executor.config.CreateUrl(doc.ID)))

		if err != nil {
			return "", false
		}

		// Check if user chose to skip
		if selectedOption == "Skip this document" {
			return "", false
		}

		// Find the selected caption
		for i, option := range options[:len(captionResp.Captions)] {
			if option == selectedOption {
				return captionResp.Captions[i].Caption, true
			}
		}

		return "", false
	})
}

// SetContentAction - Set document content which content contains pattern
type SetContentAction struct{}

func (a *SetContentAction) Description() string {
	return "Set document content which content contains pattern"
}

func (a *SetContentAction) Execute(executor *ActionExecutor) error {
	// Get all documents
	pterm.Info.Println("Fetching documents from Paperless NGX...")
	documents, err := executor.paperlessClient.GetDocuments()
	if err != nil {
		return fmt.Errorf("failed to get documents: %w", err)
	}
	pterm.Success.Printf("Fetched %d documents\n", len(documents))

	// Filter documents by content pattern
	pterm.Info.Println("Filtering documents by content pattern...")
	filteredDocs, err := executor.paperlessClient.FilterDocuments(documents, "content")
	if err != nil {
		return fmt.Errorf("failed to filter documents: %w", err)
	}
	pterm.Success.Printf("Found %d documents matching content patterns\n", len(filteredDocs))

	if len(filteredDocs) == 0 {
		pterm.Warning.Println("No documents found matching the content patterns")
		return nil
	}

	// Display bar chart with document counts
	bars := []pterm.Bar{
		{Label: "All", Value: len(documents), Style: pterm.NewStyle(pterm.FgGray)},
		{Label: "Found", Value: len(filteredDocs), Style: pterm.NewStyle(pterm.FgGreen)},
	}
	pterm.DefaultBarChart.WithHorizontal().WithBars(bars).WithShowValue().Render()

	// Ask for confirmation
	confirmed, err := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(false).
		WithDefaultText("Do you want to extract content for these documents using LLM?").
		Show()
	if err != nil {
		return fmt.Errorf("failed to get confirmation: %w", err)
	}

	if !confirmed {
		pterm.Info.Println("Operation cancelled by user")
		return nil
	}

	// Process documents
	return executor.processOCRGeneration(filteredDocs, func(doc Document, newContent string, newTitle string) bool {
		previewContent := newContent
		if len(newContent) > 50 {
			previewContent = newContent[:50] + "..."
		}

		// Ask for user confirmation
		confirmed, err := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(false).
			WithDefaultText(fmt.Sprintf(
				"Do you want to change the content of document '%s' to '%v' chars (title could be '%s')?\n"+
					"Url: %s\n"+
					"First 50 chars: %s\n"+
					"Change content?",
				doc.Title, len(newContent), newTitle, executor.config.CreateUrl(doc.ID), previewContent,
			)).
			Show()
		if err != nil {
			return false
		}
		return confirmed
	})
}

func (e *ActionExecutor) processOCRGeneration(documents []Document, userCallback func(Document, string, string) bool) error {
	stats := &ProgressStats{
		processed: 0,
		success:   0,
		errors:    0,
		skipped:   0,
		total:     len(documents),
	}

	pterm.Info.Println("Starting OCR generation process...")

	for _, doc := range documents {
		// Download document pdf
		pdfBytes, err := e.paperlessClient.DownloadDocument(doc.ID)
		if err != nil {
			pterm.Warning.Printf("Failed to download PDF for document %d: %v\n", doc.ID, err)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		// Convert first page to JPEG
		jpegData, err := e.config.RenderPageToJpg(pdfBytes, 0)
		if err != nil {
			pterm.Warning.Printf("Failed to render page to JPG for document %d: %v\n", doc.ID, err)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		// Extract content using LLM
		newContent, err := e.llmClient.ExtractContent(jpegData)
		if err != nil {
			pterm.Warning.Printf("Failed to extract content for document %d: %v\n", doc.ID, err)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		// Generate new titles using LLM
		captions, err := e.llmClient.GenerateTitleFromContent(newContent)
		if err != nil {
			pterm.Warning.Printf("Failed to generate title for document %d: %v\n", doc.ID, err)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		if len(captions.Captions) == 0 {
			pterm.Warning.Printf("No titles generated for document %d\n", doc.ID)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		// Sort captions by score (highest score first)
		sort.Slice(captions.Captions, func(i, j int) bool {
			return captions.Captions[i].Score > captions.Captions[j].Score
		})

		newTitle := captions.Captions[0].Caption

		if userCallback != nil {
			pterm.Info.Println("Start User Interaction")
			if !userCallback(doc, newContent, newTitle) {
				pterm.Warning.Println("User cancelled this operation")
				stats.skipped++
				stats.processed++
				stats.renderProgressChart()
				continue
			}
			pterm.Info.Println("End of User Interaction")
		}

		// Update document content
		updates := map[string]interface{}{
			"content": newContent,
		}

		if err := e.paperlessClient.UpdateDocument(doc.ID, updates); err != nil {
			pterm.Warning.Printf("Failed to update document %d: %v\n", doc.ID, err)
			stats.errors++
		} else {
			stats.success++
		}

		stats.processed++
		stats.renderProgressChart()
	}

	stats.renderFinalSummary(len(documents))
	return nil
}

// SetTitleAndContentAction - Set document content and title which contains pattern
type SetTitleAndContentAction struct{}

func (a *SetTitleAndContentAction) Description() string {
	return "Set document content and title which contains pattern"
}

func (a *SetTitleAndContentAction) Execute(executor *ActionExecutor) error {
	pterm.Warning.Println("Combined title and content processing is not yet implemented")
	return nil
}

// SetTitleAndContentWithLLMAction - Set document content and title which contains LLM response contains pattern
type SetTitleAndContentWithLLMAction struct{}

func (a *SetTitleAndContentWithLLMAction) Description() string {
	return "Set document content and title which contains LLM response contains pattern"
}

func (a *SetTitleAndContentWithLLMAction) Execute(executor *ActionExecutor) error {
	pterm.Warning.Println("LLM response pattern matching is not yet implemented")
	return nil
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

func (e *ActionExecutor) processDocumentsForTitleGeneration(documents []Document, userCallback func(Document, CaptionResponse) (string, bool)) error {
	stats := &ProgressStats{
		processed: 0,
		success:   0,
		errors:    0,
		skipped:   0,
		total:     len(documents),
	}

	pterm.Info.Println("Starting title generation process...")

	for _, doc := range documents {
		// Generate new titles using LLM
		captions, err := e.llmClient.GenerateTitleFromContent(doc.Content)
		if err != nil {
			pterm.Warning.Printf("Failed to generate title for document %d: %v\n", doc.ID, err)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		if len(captions.Captions) == 0 {
			pterm.Warning.Printf("No titles generated for document %d\n", doc.ID)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		// Sort captions by score (highest score first)
		sort.Slice(captions.Captions, func(i, j int) bool {
			return captions.Captions[i].Score > captions.Captions[j].Score
		})

		// Check if any captions need rescanning
		needsRescan := false
		for _, caption := range captions.Captions {
			if caption.Caption == "RESCAN DOCUMENT" {
				needsRescan = true
				break
			}
		}

		if needsRescan {
			pterm.Warning.Printf("Document %d needs rescanning, skipping\n", doc.ID)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		var selectedTitle string
		var userConfirmed bool

		if userCallback != nil {
			pterm.Info.Println("Start User Interaction")
			selectedTitle, userConfirmed = userCallback(doc, captions)
			if !userConfirmed {
				pterm.Warning.Println("User cancelled this operation")
				stats.skipped++
				stats.processed++
				stats.renderProgressChart()
				continue
			}
			pterm.Info.Println("End of User Interaction")
		} else {
			// Use the first generated title if no callback
			selectedTitle = captions.Captions[0].Caption
			userConfirmed = true
		}

		// Update document title
		updates := map[string]interface{}{
			"title": selectedTitle,
		}

		if err := e.paperlessClient.UpdateDocument(doc.ID, updates); err != nil {
			pterm.Warning.Printf("Failed to update document %d: %v\n", doc.ID, err)
			stats.errors++
		} else {
			stats.success++
		}

		stats.processed++
		stats.renderProgressChart()
	}

	stats.renderFinalSummary(len(documents))
	return nil
}
