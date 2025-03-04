# Rule Service

A FastAPI-based service that provides various content analysis and filtering capabilities through a plugin system. The service includes plugins for:

- Content safety and moderation
- English language detection
- Invisible character detection
- PII (Personally Identifiable Information) detection
- Prompt injection detection
- Azure AI content safety

## Prerequisites

- Python 3.11+
- Poetry (Python package manager)
- Docker (optional, for containerized deployment)

## Environment Setup

1. Clone the repository and navigate to the rule service directory:
```bash
cd services/rule
```

2. Create a `.env` file in the root directory with the following variables:
```
OPENAI_API_KEY=your_openai_api_key
CONTENT_SAFETY_KEY=your_azure_content_safety_key
CONTENT_SAFETY_ENDPOINT=your_azure_content_safety_endpoint
HUGGINGFACE_API_KEY=your_huggingface_api_key
LOG_LEVEL=INFO  # Optional, defaults to INFO
```

3. Install dependencies using Poetry:
```bash
poetry install
```

## Running Locally

1. Activate the Poetry virtual environment:
```bash
poetry shell
```

2. Start the FastAPI server:
```bash
python src/main.py
```

The service will be available at `http://localhost:8000`

## API Endpoints

- Health Check: `GET /status/healthz`
- Rule Execution: `POST /rule/execute`

### Example Request

```json
{
    "prompt": {
        "messages": [
            {
                "role": "user",
                "content": "Hello, how are you?"
            }
        ]
    },
    "config": {
        "PluginName": "content_safety",
        "Threshold": 0.5,
        "Relation": ">"
    }
}
```

## Docker Deployment

1. Build the Docker image:
```bash
docker build -t rule-service .
```

2. Run the container:
```bash
docker run -p 8000:8000 --env-file .env rule-service
```

## Running Tests
```bash
python -m unittest src/tests/test_api.py
```

## Available Plugins

- `azure_ai_content_safety_prompt_injection`: Uses Azure Content Safety API for prompt injection detection
- `content_safety`: Analyzes text for inappropriate content
- `detect_english`: Detects if text is in English
- `invisible_chars`: Detects invisible Unicode characters
- `openai_moderation`: Uses OpenAI's moderation API
- `pii`: Detects and anonymizes personal information
- `prompt_injection_llm`: Uses ML models to detect prompt injection attempts

## Contributing

1. Create a new plugin in `src/plugins/`
2. Implement the required `handler(text: str, threshold: float, config: dict)` function
3. Add tests in `src/tests/`
4. Update documentation as needed

## License

MIT License
