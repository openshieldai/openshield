# OpenShield - Firewall for AI models


>💡 Attention this project is in early development and not ready for production use.


## Why do you need this?
AI models a new attack vector for hackers. They can use AI models to generate malicious content, spam, or phishing attacks. OpenShield is a firewall for AI models. It provides rate limiting, content filtering, and keyword filtering for AI models. It also provides a tokenizer calculation for OpenAI models.

## Solution
OpenShield a transparent proxy that sits between your AI model and the client. It provides rate limiting, content filtering, and keyword filtering for AI models.

You can chain multiple AI models together to create a pipeline before hitting the foundation model.

## Features
- You can set custom rate limits for OpenAI endpoints
- Tokenizer calculation for OpenAI models
- Python and LLM based rules

## Incoming features
- Rate limiting per user
- Rate limiting per model
- Prompts manager
- Content filtering / Keyword filtering based by LLM/Vector models
- OpenMeter integration
- VectorDB integration

## Requirements
- OpenAI API key
- Postgres
- Redis

#### OpenShield is a firewall designed for AI models.


### Endpoints
```
/openai/v1/models
/openai/v1/models/:model
/openai/v1/chat/completions
```

#### Output:
```
{"model":"gpt-3.5","prompts":"thisateststringfortokenizer","tokens":6}
```


## Local development
.env is supported in local development. Create a .env file in the root directory with the following content:
```
ENV=development go run main.go
```

## Local test

Generate hash from your OpenAI API key:
```
echo -n "mykey" | openssl dgst -sha256 | awk '{print $2}'
```

Change the `SETTINGS_OPENAI_API_KEY_HASH` in the `docker-compose.yml` file to your OpenAI API key.

```
cd docker
docker-compose build
docker-compose up
curl -vvv --location 'localhost:8080/openai/v1/chat/completions' \                                                                                       
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer <yourapikey>' \
--data '{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"What is the meaning of life?"}]}'
```

## Example test-client

```
npm install
npx tsc src/index.ts
export OPENAI_API_KEY=<yourapikey>
node src/index.js
```
