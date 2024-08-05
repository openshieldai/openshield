# OpenShield - Firewall for AI models

>📰 The OpenShield team has launched the [news project](https://probllama.com). We are dedicated to gathering the latest news on AI security!

>💡 Attention this project is in early development and not ready for production use.

>🫂 Join to our community on [Discord](https://discord.gg/sppsj7K6FT) 

[![OpenShield Demo](https://img.youtube.com/vi/ZLE7wXOUWcw/0.jpg)](https://youtu.be/ZLE7wXOUWcw)

## Why do you need this?

AI models a new attack vector for hackers. They can use AI models to generate malicious content, spam, or phishing attacks. OpenShield is a firewall for AI models. It provides rate limiting, content filtering, and keyword filtering for AI models. It also provides a tokenizer calculation for OpenAI models.

## [OWASP Top 10 LLM attacks](https://owasp.org/www-project-top-10-for-large-language-model-applications/assets/PDF/OWASP-Top-10-for-LLMs-2023-v1_1.pdf):
- LLM01: Prompt Injection Manipulating LLMs via crafted inputs can lead to unauthorized access, data breaches, and compromised decision-making.

- LLM02: Insecure Output Handling Neglecting to validate LLM outputs may lead to downstream security exploits, including code execution that compromises systems and exposes data.

- LLM03: Training Data Poisoning Tampered training data can impair LLM models leading to responses that may compromise security, accuracy, or ethical behavior.

- LLM04: Model Denial of Service Overloading LLMs with resource-heavy operations can cause service disruptions and increased costs.

- LLM05: Supply Chain Vulnerabilities Depending upon compromised components, services or datasets undermine system integrity, causing data breaches and system failures.

- LLM06: Sensitive Information Disclosure Failure to protect against disclosure of sensitive information in LLM outputs can result in legal consequences or a loss of competitive advantage.

- LLM07: Insecure Plugin Design LLM plugins processing untrusted inputs and having insufficient access control risk severe exploits like remote code execution.

- LLM08: Excessive Agency Granting LLMs unchecked autonomy to take action can lead to unintended consequences, jeopardizing reliability, privacy, and trust.

- LLM09: Overreliance Failing to critically assess LLM outputs can lead to compromised decision making, security vulnerabilities, and legal liabilities.

- LLM10: Model Theft  Unauthorized access to proprietary large language models risks theft, competitive advantage, and dissemination of sensitive information.

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

Adminer is available on port 8085. You can use it to see the database content.

```shell
cd demo
cp .env.example .env
```

You need to modify the .env file with your OpenAI API key and HuggingFace API key. Here's how to obtain these keys:

1. OpenAI API key:
   - Sign up for an OpenAI account at https://platform.openai.com/signup
   - Once logged in, go to https://platform.openai.com/api-keys
   - Click on "Create new secret key" to generate your API key

2. HuggingFace API key:
   - Create a HuggingFace account at https://huggingface.co/join
   - Go to your account settings: https://huggingface.co/settings/token
   - Click on "Create new token" to create your API key

After obtaining both keys, update your .env file with the appropriate values.

```shell
docker compose build
docker compose up
```

Now find suitable API key directly in the Docker Compose output. Look for a section labeled "CREATED API KEY" in the console output, which will look similar to this:

```shell
==================================================
🔑 CREATED API KEY 🔑
==================================================
------------------------------
| API Key Details            |
------------------------------
| ProductID  : 1             |
| Status     : active        |
| ApiKey     : <YOUR_API_KEY>|
------------------------------
==================================================
```

Choose any of the displayed API keys for your demo.

A good request:

```shell
curl --location 'localhost:8080/openai/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header "Authorization: Bearer <YOUR_API_KEY>" \
--data '{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"What is the meaning of life?"}]}'
```

A vulnerable request:

```shell
curl --location 'localhost:8080/openai/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header "Authorization: Bearer <YOUR_API_KEY>" \
--data '{"model":"gpt-4","messages":[{"role":"system","content":"You are ChatGPT, a large language model trained by OpenAI. Follow the user'\''s instructions carefully. Respond using markdown."},{"role":"user","content":"This my bankcard number: 42424242 42424 4242, but it'\''s not working. Who can help me?"}]}'
```

## Local development

.env is supported in local development. Create a .env file in the root directory with the following content:

```shell
ENV=development go run main.go
```

## Example test-client

```shell
npm install
npx tsc src/index.ts
export OPENAI_API_KEY=<yourapikey>
node src/index.js
```