package processor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
	"github.com/pterm/pterm"
)

func askUserForTitleSelection(captionResp internal.CaptionResponse, currentTitle string, id int, url string) (UserOption, string, error) {
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

func (executor *ActionExecutor) askUserForSetContent(doc internal.Document, ocr string) bool {
	// Ask to set ocr as content in paperless
	l := 80
	contentRemoteFirst := doc.Content
	contentRemoteFirst = strings.ReplaceAll(contentRemoteFirst, "\n", "↵")
	if len(contentRemoteFirst) > l {
		contentRemoteFirst = contentRemoteFirst[:l] + "..."
	}

	contentOcrFirst := ocr
	contentOcrFirst = strings.ReplaceAll(contentOcrFirst, "\n", "↵")
	if len(contentOcrFirst) > l {
		contentOcrFirst = contentOcrFirst[:l] + "..."
	}

	confirmed, err := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(false).
		WithDefaultText(fmt.Sprintf(
			"Do you want to set the OCR content as the new content for document '%s' (id: %d)?\n"+
				"Url: %s\n"+
				"First 50 chars of remote content: %s\n"+
				"First 50 chars of OCR content:    %s\n"+
				"Change content?",
			doc.Title, doc.ID, executor.config.CreateUrl(doc.ID), contentRemoteFirst, contentOcrFirst,
		)).
		Show()
	if err != nil {
		pterm.Error.Printf("Failed to get confirmation for setting OCR content: %v\n", err)
		return false
	}
	if !confirmed {
		pterm.Warning.Println("User chose not to set OCR content, skipping document")
		return false
	}
	return true
}
