[![Go](https://github.com/dhcgn/paperless-ngx-privatemode-ai/actions/workflows/build_and_test.yml/badge.svg)](https://github.com/dhcgn/paperless-ngx-privatemode-ai/actions/workflows/build_and_test.yml)

⚠️ This project is in early development stage. It may not work as expected and may change in future releases.
⚠️ Use it at your own risk, no warranty of any kind is provided.

This project is for demonstration purposes only. It is not affiliated with or endorsed by Paperless NGX or Privatemode.ai.

# paperless-ngx-privatemode-ai

Help with documents in [Paperless NGX](https://docs.paperless-ngx.com/) with confidential LLM by [Privatemode.ai](https://privatemode.ai) (docs https://docs.privatemode.ai/).

## Motivation

Managing documents with [Paperless NGX](https://docs.paperless-ngx.com/) is powerful, but I wanted to enhance privacy and automation by integrating a confidential LLM. My goal is to automatically generate accurate titles and content for documents—without exposing sensitive data to public LLM providers like OpenAI or Google.

While Paperless NGX uses OCRmyPDF with Tesseract for text extraction, its results are sometimes imperfect. Leveraging a private LLM allows for improved title suggestions and content extraction, all within a secure environment.

If this approach proves valuable for myself, I plan to expand the tool to support automatic tagging and additional metadata.

## GUI

![Screenshot](docs/screenshot.png)

## Run

```cmd
paperless-ngx-privatemode-ai.exe --config config.yaml
```

### Config

See [config.yaml](config.yaml) for example configuration.

You need to install imagemagick on your system an set the path in this configuration file.
I'm not happy with this solution, but it works for now. I was not able to find a native Go library to convert pdf to images.

### Dialog

```cmd
# Set content or title of documents in Paperless NGX with confidential LLM by Privatemode.ai

1. Set document titles which title contains pattern
2. Set document content which content contains pattern
3. Set document content and title which contains pattern **(Not implemented yet)**
4. Set document content and title which contains LLM response contains pattern **(Not implemented yet)**
```

### Program flow

1. Load configuration from argument `--config`
2. Check configuration
3. Check if `paperless-ngx` is accessible
4. Check if `privatemode.ai` is accessible and models are available
5. Ask user for action
6. Execute action and show progress


## More Use Cases with Privatemode.ai

Go to my collection of ai scripts: https://github.com/dhcgn/ai-sample-scripts
