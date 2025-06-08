package processor

import (
	"fmt"
	"sort"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
	"github.com/pterm/pterm"
)

// SetOcrInContentAction - Set document content which content contains pattern
type SetOcrInContentAction struct{}

func (a *SetOcrInContentAction) Description() string {
	return "Set document content which content contains pattern using OCR"
}

func (a *SetOcrInContentAction) Execute(executor *ActionExecutor) error {
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
		WithDefaultText("Do you want to make ocr for these documents using LLM?").
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
		return executor.askUserForSetContent(doc, newContent)
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
		pterm.Info.Printf("Generating ocr for document '%s' (id: %d, link: %s)\n", doc.Title, doc.ID, e.config.CreateUrl(doc.ID))

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

		if err := SetContentOfPaperlessDocument(e, doc.ID, newContent); err != nil {
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
