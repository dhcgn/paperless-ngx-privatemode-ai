package processor

import (
	"fmt"
	"sort"
	"strings"

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
	filteredDocs, err := executor.paperlessClient.FilterDocuments(documents, internal.FilterTypeTitle)
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
	return executor.processDocumentsForTitleGeneration(filteredDocs, func(doc internal.Document, captionResp internal.CaptionResponse) (string, bool) {
		// Show document summary first
		if captionResp.Summarize != "" {
			pterm.Info.Printf("Document Summary: %s\n\n", captionResp.Summarize)
		}

	AskForTitleSelection:
		selectedOption, userSelectedTitle, err := AskForTitleSelection(captionResp, doc.Title, doc.ID, executor.config.CreateUrl(doc.ID))
		if err != nil {
			return "", false
		}
		if userSelectedTitle != "" {
			return userSelectedTitle, true
		}

		// Check if user chose to skip
		if selectedOption == skipDocumentOption {
			return "", false
		}

		// Check if user chose to make OCR and try again
		if selectedOption == makeOcrAndTryAgainOption {
			pterm.Info.Println("Making OCR and trying again...")
			// Call the OCR generation process
			c := HeadlessActionClients{
				Config:          executor.config,
				PaperlessClient: executor.paperlessClient,
				LLMClient:       executor.llmClient,
			}
			_, captions, err := c.OcrPaperlessDocument(doc.ID, func(status string) {
				pterm.Info.Println(status)
			})
			if err != nil {
				pterm.Error.Printf("Failed to make OCR and generate title: %v\n", err)
				return "", false
			}
			if len(captions.Captions) == 0 {
				pterm.Warning.Println("No titles generated after OCR, skipping document")
				return "", false
			}
			captionResp = *captions

			goto AskForTitleSelection // Re-ask for title selection with new captions

		}

		return "", false
	})
}

func AskForTitleSelection(captionResp internal.CaptionResponse, currentTitle string, id int, url string) (UserOption, string, error) {
	// Sort captions by score (highest score first)
	sort.Slice(captionResp.Captions, func(i, j int) bool {
		return captionResp.Captions[i].Score > captionResp.Captions[j].Score
	})

	mapTitleToOptions := make(map[string]string)

	// Prepare options for user selection
	options := make([]string, 0, len(captionResp.Captions)+2)

	// Add each caption with its score
	for i, caption := range captionResp.Captions {
		optDisplayTitleWithScore := fmt.Sprintf("%d. %s (Score: %.2f)", i+1, caption.Caption, caption.Score)
		options = append(options, optDisplayTitleWithScore)
		mapTitleToOptions[optDisplayTitleWithScore] = caption.Caption
	}

	// Add option for custom title
	options = append(options, string(customTitleOption))

	// Add option to skip
	options = append(options, string(skipDocumentOption))

	// Add option to make OCR and try again
	options = append(options, string(makeOcrAndTryAgainOption))

	// Show interactive select
	selectedOption, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultOption(string(skipDocumentOption)).
		Show(fmt.Sprintf("Choose a new title for document '%s' (id: %d):\nUrl: %s\n", currentTitle, id, url))

	if err != nil {
		return Undefined, "", fmt.Errorf("failed to get user selection: %w", err)
	}

	// Check if the selected option is valid
	if selectedOption == "" {
		return Undefined, "", fmt.Errorf("no valid option selected")
	}

	// Declare userOption before the switch
	var userOption UserOption

	// Check if the selected option is one of the custom options using a switch for clarity
	switch selectedOption {
	case string(skipDocumentOption):
		userOption = skipDocumentOption
	case string(makeOcrAndTryAgainOption):
		userOption = makeOcrAndTryAgainOption
	case string(customTitleOption):
		userOption = customTitleOption
	default:
		userOption = Undefined
	}

	customUserTitle := ""
	if userOption == customTitleOption {
		pterm.Println()
		pterm.Info.Println("Please enter your custom title:")

		// Create an interactive text input with single line input mode and show it
		result, err := pterm.DefaultInteractiveTextInput.Show()
		if err != nil {
			pterm.Error.Printf("Failed to get custom title input: %v\n", err)
			return userOption, "", fmt.Errorf("failed to get custom title input: %w", err)
		}

		// Print a blank line for better readability
		pterm.Println()

		// Check if user entered something
		if strings.TrimSpace(result) == "" {
			pterm.Warning.Println("No title entered, skipping document")
			return userOption, "", nil
		}

		// Print the user's answer with an info prefix
		pterm.Info.Printfln("You entered: %s", result)

		return userOption, strings.TrimSpace(result), nil
	} else if userOption != Undefined {
		return userOption, "", nil
	}

	// If the selected option is one of the captions, return the corresponding title
	customUserTitle, exists := mapTitleToOptions[selectedOption]
	if !exists {
		return Undefined, "", fmt.Errorf("selected option '%s' does not correspond to any title", selectedOption)
	}
	// If the selected option is a valid caption, return it
	if customUserTitle == "" {
		return Undefined, "", fmt.Errorf("no title found for selected option '%s'", selectedOption)
	}

	return userOption, customUserTitle, nil
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
	filteredDocs, err := executor.paperlessClient.FilterDocuments(documents, internal.FilterTypeContent)
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
	return executor.processOCRGeneration(filteredDocs, func(doc internal.Document, newContent string, newTitle string) bool {
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

func (e *ActionExecutor) processOCRGeneration(documents []internal.Document, userCallback func(internal.Document, string, string) bool) error {
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
		jpegData, err := internal.RenderPageToJpg(e.config, pdfBytes, 0)
		if err != nil {
			pterm.Warning.Printf("Failed to render page to JPG for document %d: %v\n", doc.ID, err)
			stats.errors++
			stats.processed++
			stats.renderProgressChart()
			continue
		}

		// Extract content using LLM
		newContent, err := e.llmClient.MakeOcr(jpegData)
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

func (e *ActionExecutor) processDocumentsForTitleGeneration(documents []internal.Document, userCallback func(internal.Document, internal.CaptionResponse) (string, bool)) error {
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
		pterm.Info.Printf("Generating title for document '%s' (id: %d, link: %s)\n", doc.Title, doc.ID, e.config.CreateUrl(doc.ID))
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
