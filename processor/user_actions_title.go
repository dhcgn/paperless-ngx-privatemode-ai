package processor

import (
	"fmt"
	"sort"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
	"github.com/pterm/pterm"
)

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
	if !executor.autonomous {
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
	}

	// Process documents
	return executor.processDocumentsForTitleGeneration(filteredDocs, func(doc internal.Document, captionResp internal.CaptionResponse) (string, bool) {
		// Show document summary first
		if captionResp.Summarize != "" {
			pterm.Info.Printf("Document Summary: %s\n\n", captionResp.Summarize)
		}

	AskForTitleSelection:
		selectedOption, userSelectedTitle, err := askUserForTitleSelection(captionResp, doc.Title, doc.ID, executor.config.CreateUrl(doc.ID))
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

			ocr, captions, err := executor.OcrPaperlessDocument(doc.ID, func(status string) {
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

			if executor.askUserForSetContent(doc, ocr) {
				err := SetContentOfPaperlessDocument(executor, doc.ID, ocr)
				if err != nil {
					pterm.Error.Printf("Failed to set content for document %d: %v\n", doc.ID, err)
					return "", false
				}
				pterm.Success.Printf("Content set for document '%s' (id: %d)\n", doc.Title, doc.ID)
			}
			goto AskForTitleSelection // Re-ask for title selection with new captions

		}

		return "", false
	})
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

		if userCallback != nil && !e.autonomous {
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

			if e.autonomous {
				selectedTitle = selectedTitle + " (auto-generated)"
			}

			userConfirmed = true
		}

		if err := SetTitleOfPaperlessDocument(e, doc.ID, selectedTitle); err != nil {
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
