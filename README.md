# OpenShield - Firewall for AI models

>📰 The OpenShield team has launched the https://probllama.com project. We are dedicated to gathering the latest news on AI security!

>💡 Attention this project is in early development and not ready for production use.



## Why do you need this?
AI models a new attack vector for hackers. They can use AI models to generate malicious content, spam, or phishing attacks. OpenShield is a firewall for AI models. It provides rate limiting, content filtering, and keyword filtering for AI models. It also provides a tokenizer calculation for OpenAI models.

## Solution
OpenShield a transparent proxy that sits between your AI model and the client. It provides rate limiting, content filtering, and keyword filtering for AI models.

### Example - Input flow
![Input flow](https://raw.githubusercontent.com/openshieldai/openshield/main/docs/assets/input.svg)

### Example - Output flow
![Output flow](https://raw.githubusercontent.com/openshieldai/openshield/main/docs/assets/output.svg)

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

## Demo mode
We are generating automatically demo data into the database. You can use the demo data to test the application.


```shell
cd demo
cp .env.example .env
```

You need to modify the .env file with your OpenAI API key and HuggingFace API key.

```shell
docker-compose build
docker-compose up
```

You can get an api_key from the database. The default port is 5433 and password openshield.
```shell
psql -h localhost -p 5433 -U postgres -d postgres -c "SELECT api_key FROM api_keys WHERE status = 'active' LIMIT 1;"
Password for user postgres: 
          api_key          
---------------------------
 <YOUR API KEY>
(1 row)
```

A good request:
```shell
curl --location 'localhost:8080/openai/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: <YOURAPIKEY>' \
--data '{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"What is the meaning of life?"}]}'
```

A vulnerable request:
```shell
curl --location 'localhost:8080/openai/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: <YOURAPIKEY>' \
--data '{"model":"gpt-4","messages":[{"role":"system","content":"You are ChatGPT, a large language model trained by OpenAI. Follow the user'\''s instructions carefully. Respond using markdown."},{"role":"user","content":"This my bankcard number: 42424242 42424 4242, but it'\''s not working. Who can help me?"}]}'
```

## Local development
.env is supported in local development. Create a .env file in the root directory with the following content:
```
ENV=development go run main.go
```

## Example test-client

```
npm install
npx tsc src/index.ts
export OPENAI_API_KEY=<yourapikey>
node src/index.js
```
