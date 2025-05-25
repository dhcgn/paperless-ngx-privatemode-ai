⚠️ This project is in early development stage. It may not work as expected and may change in future releases.
⚠️ Use it at your own risk, no warranty of any kind is provided.

# paperless-ngx-privatemode-ai

Help with documents in [Paperless NGX](https://docs.paperless-ngx.com/) with confidential LLM by [Privatemode.ai](https://privatemode.ai) (docs https://docs.privatemode.ai/).

## Run

```cmd
paperless-ngx-privatemode-ai.exe --config config.yaml
```

### Config

See [config.yaml](config.yaml) for example configuration.

### Dialog

```cmd
# Set content or title of documents in Paperless NGX with confidential LLM by Privatemode.ai

1. Set document titles which title contains pattern
2. Set document content which content contains pattern
3. Set document content and title which contains pattern
4. Set document content and title which contains LLM response contains pattern
```

### Program flow

1. Load configuration from argument `--config`
2. Check configuration
3. Check if `paperless-ngx` is accessible
4. Check if `privatemode.ai` is accessible and models are available
5. Ask user for action
6. Execute action and show progress