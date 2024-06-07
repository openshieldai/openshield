# OpenShield - Firewall for AI models

#### OpenShield is a firewall designed for AI models.

- `ENV`: Specifies the environment in which the application is running. Possible values are `testing` and `production`.
- `TESTING_OPENAI_API_KEY`: The API key for OpenAI used in testing environment.
- `SETTINGS_LOG_DISABLE_COLORS`: If set to `true`, disables color output in the logs.
- `SETTINGS_OPENAI_API_KEY_HASH`: The hashed version of the OpenAI API key.
- `SETTINGS_ROUTES_OPENAI_MODELS_MAX`: Maximum number of OpenAI models that can be used.
- `SETTINGS_ROUTES_OPENAI_MODEL_MAX`: Maximum number of a specific OpenAI model that can be used.
- `SETTINGS_ROUTES_OPENAI_MODEL_EXPIRATION`: Expiration time for a specific OpenAI model.
- `SETTINGS_ROUTES_OPENAI_MODELS_EXPIRATION`: Expiration time for all OpenAI models.
- `SETTINGS_ROUTES_OPENAI_MODEL_TIME`: Time limit for using a specific OpenAI model.
- `SETTINGS_ROUTES_OPENAI_MODELS_TIME`: Time limit for using all OpenAI models.
- `SETTINGS_ROUTES_OPENAI_CHAT_COMPLETIONS_MAX`: Maximum number of chat completions that can be made with OpenAI.
- `SETTINGS_ROUTES_OPENAI_CHAT_COMPLETIONS_EXPIRATION`: Expiration time for chat completions with OpenAI.
- `SETTINGS_ROUTES_OPENAI_CHAT_COMPLETIONS_TIME`: Time limit for chat completions with OpenAI.
- `SETTINGS_ROUTES_OPENAI_TOKENIZER_MAX`: Maximum number of tokenizations that can be made with OpenAI.
- `SETTINGS_ROUTES_OPENAI_TOKENIZER_EXPIRATION`: Expiration time for tokenizations with OpenAI.
- `SETTINGS_ROUTES_OPENAI_TOKENIZER_TIME`: Time limit for tokenizations with OpenAI.
- `SETTINGS_OPENSHIELD_API_KEY`: The API key for OpenShield.
- `SETTINGS_ROUTES_STORAGE_REDIS_URL`: The URL for the Redis storage used by the application.
- `SETTINGS_ROUTES_STORAGE_REDIS_TLS`: If set to `true`, enables TLS for the Redis connection.

### Endpoints
```
/openai/v1/models
/openai/v1/models/:model
/openai/v1/chat/completions
/tokenizer/:model -> Using OPENSHIELD_API_KEY
```
### /tokenizer/:model:
#### Input (POST)
```
thisateststringfortokenizer
```

#### Output:
```
{"model":"gpt-3.5","prompts":"thisateststringfortokenizer","tokens":6}
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
