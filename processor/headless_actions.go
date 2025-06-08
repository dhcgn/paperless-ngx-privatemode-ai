package processor

import (
	"errors"

	"github.com/dhcgn/paperless-ngx-privatemode-ai/config"
	"github.com/dhcgn/paperless-ngx-privatemode-ai/internal"
)

type HeadlessActionClients struct {
	Config          *config.Config
	PaperlessClient *internal.PaperlessClient
	LLMClient       *internal.LLMClient
}

func (clients HeadlessActionClients) OcrPaperlessDocument(documentID int, statusCallback func(string)) (string, *internal.CaptionResponse, error) {
	if documentID <= 0 {
		return "", nil, errors.New("invalid document ID")
	}
	if clients.PaperlessClient == nil || clients.LLMClient == nil {
		return "", nil, errors.New("clients not initialized")
	}
	if clients.Config == nil {
		return "", nil, errors.New("config not initialized")
	}

	if statusCallback != nil {
		statusCallback("Downloading PDF document...")
	}
	pdfBytes, err := clients.PaperlessClient.DownloadDocument(documentID)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to download PDF"))
	}

	if statusCallback != nil {
		statusCallback("Converting PDF to JPEG...")
	}
	jpegData, err := internal.RenderPageToJpg(clients.Config, pdfBytes, 0)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to render page to JPG"))
	}

	if statusCallback != nil {
		statusCallback("Making OCR from JPEG...")
	}
	newContent, err := clients.LLMClient.MakeOcr(jpegData)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to make OCR"))
	}

	if statusCallback != nil {
		statusCallback("Generating title from content...")
	}
	captions, err := clients.LLMClient.GenerateTitleFromContent(newContent)
	if err != nil {
		return "", nil, errors.Join(err, errors.New("failed to generate title"))
	}
	return newContent, &captions, nil
}
