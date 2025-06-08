package processor

import (
	"errors"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
)

type Paperless interface {
	GetPaperlessClient() *internal.PaperlessClient
}

func (clients ActionExecutor) GetPaperlessClient() *internal.PaperlessClient {
	if clients.paperlessClient == nil {
		return nil
	}
	return clients.paperlessClient
}

func SetContentOfPaperlessDocument(instances Paperless, documentID int, content string) error {
	if documentID <= 0 {
		return errors.New("invalid document ID")
	}

	if content == "" {
		return errors.New("content cannot be empty")
	}

	// Update document content
	updates := map[string]interface{}{
		"content": content,
	}

	err := instances.GetPaperlessClient().UpdateDocument(documentID, updates)
	if err != nil {
		return errors.Join(err, errors.New("failed to set document content"))
	}
	return nil
}

func SetTitleOfPaperlessDocument(instances Paperless, documentID int, title string) error {
	if documentID <= 0 {
		return errors.New("invalid document ID")
	}

	if title == "" {
		return errors.New("title cannot be empty")
	}

	// Update document content
	updates := map[string]interface{}{
		"title": title,
	}

	err := instances.GetPaperlessClient().UpdateDocument(documentID, updates)
	if err != nil {
		return errors.Join(err, errors.New("failed to set document title"))
	}
	return nil
}

func (clients ActionExecutor) OcrPaperlessDocument(documentID int, statusCallback func(string)) (string, *internal.CaptionResponse, error) {
	if documentID <= 0 {
		return "", nil, errors.New("invalid document ID")
	}
	if clients.paperlessClient == nil || clients.llmClient == nil {
		return "", nil, errors.New("clients not initialized")
	}
	if clients.config == nil {
		return "", nil, errors.New("config not initialized")
	}

	if statusCallback != nil {
		statusCallback("Downloading PDF document...")
	}
	pdfBytes, err := clients.paperlessClient.DownloadDocument(documentID)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to download PDF"))
	}

	if statusCallback != nil {
		statusCallback("Converting PDF to JPEG...")
	}
	jpegData, err := internal.RenderPageToJpg(clients.config, pdfBytes, 0)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to render page to JPG"))
	}

	if statusCallback != nil {
		statusCallback("Making OCR from JPEG...")
	}
	newContent, err := clients.llmClient.MakeOcr(jpegData)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to make OCR"))
	}

	if statusCallback != nil {
		statusCallback("Generating title from content...")
	}
	captions, err := clients.llmClient.GenerateTitleFromContent(newContent)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to generate title"))
	}
	return newContent, &captions, nil
}
