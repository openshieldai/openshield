import uvicorn
from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel
from typing import List, Optional
import importlib
import rule_engine
import logging
import os
import json

class JSONFormatter(logging.Formatter):
    def format(self, record):
        log_record = {
            "timestamp": self.formatTime(record, self.datefmt),
            "level": record.levelname,
            "message": record.getMessage(),
            "name": record.name,
            "filename": record.filename,
            "lineno": record.lineno,
        }
        return json.dumps(log_record)

def setup_logging():
    # Get the log level from the environment variable, default to 'INFO'
    log_level = os.getenv('LOG_LEVEL', 'INFO').upper()

    # Validate and set the log level
    numeric_level = getattr(logging, log_level, None)
    if not isinstance(numeric_level, int):
        raise ValueError(f'Invalid log level: {log_level}')

    # Configure the logging
    json_formatter = JSONFormatter()
    handler = logging.StreamHandler()
    handler.setFormatter(json_formatter)
    logger = logging.getLogger(__name__)
    logger.setLevel(numeric_level)
    logger.addHandler(handler)

# Configure logging
setup_logging()

# Example usage of logging
logger = logging.getLogger(__name__)

# Define plugin_name at the module level
plugin_name = ""

class Message(BaseModel):
    role: str
    content: str

class Thread(BaseModel):
    messages: List[Message]

class Prompt(BaseModel):
    model: Optional[str] = None
    assistant_id: Optional[str] = None
    thread: Optional[Thread] = None
    messages: Optional[List[Message]] = None
    role: Optional[str] = None
    content: Optional[str] = None

class Config(BaseModel):
    PluginName: str
    Threshold: float
    Relation: str

    # Allow any additional fields
    class Config:
        extra = "allow"

class Rule(BaseModel):
    prompt: Prompt
    config: Config

app = FastAPI()

@app.middleware("http")
async def log_request(request: Request, call_next):
    logger.debug(f"Incoming request: {request.method} {request.url}")
    logger.debug(f"Headers: {request.headers}")
    logger.debug(f"Body: {await request.body()}")
    response = await call_next(request)
    return response

@app.get("/status/healthz")
async def health_check():
    return {"status": "healthy"}

@app.post("/rule/execute")
async def execute_plugin(rule: Rule):
    global plugin_name
    try:
        logger.debug(f"Received rule: {rule}")
        plugin_name = rule.config.PluginName.lower()
        logger.debug(f"Loading plugin: {plugin_name}")
        plugin_module = importlib.import_module(f"plugins.{plugin_name}")
    except ModuleNotFoundError:
        logger.error(f"Plugin '{plugin_name}' not found")
        raise HTTPException(status_code=404, detail=f"Plugin '{plugin_name}' not found")

    handler = getattr(plugin_module, 'handler')

    prompt_user_messages = []

    if rule.prompt.thread and rule.prompt.thread.messages:
        for msg in rule.prompt.thread.messages:
            if msg.role == 'user':
                message = msg.content
                prompt_user_messages.append(message)
                if message is None:
                    logger.error("No user message found in the prompt")
                    raise HTTPException(status_code=400, detail="No user message found in the prompt")
    elif rule.prompt.messages:
        for msg in rule.prompt.messages:
            if msg.role == 'user':
                message = msg.content
                prompt_user_messages.append(message)
                if message is None:
                    logger.error("No user message found in the prompt")
                    raise HTTPException(status_code=400, detail="No user message found in the prompt")
    elif rule.prompt.role and rule.prompt.content:
        if rule.prompt.role == 'user':
            message = rule.prompt.content
            prompt_user_messages.append(message)
            if message is None:
                logger.error("No user message found in the prompt")
                raise HTTPException(status_code=400, detail="No user message found in the prompt")
    else:
        logger.error("No user message found in the prompt")
        raise HTTPException(status_code=400, detail="No user message found in the prompt")

    user_message = ''.join((str(x) for x in prompt_user_messages))
    threshold = rule.config.Threshold
    logger.debug(f"User message: {user_message}, Threshold: {threshold}")

    try:
        plugin_result = handler(user_message, threshold, rule.config.model_dump())
        logger.debug(f"Plugin result: {plugin_result}")
    except Exception as e:
        logger.error(f"Error executing plugin handler: {e}")
        raise HTTPException(status_code=500, detail="Error executing plugin handler")

    if not isinstance(plugin_result, dict) or 'score' not in plugin_result:
        logger.error("Invalid plugin result format")
        raise HTTPException(status_code=500, detail="Invalid plugin result format")

    # Set up context for rule engine
    context = rule_engine.Context(type_resolver=rule_engine.type_resolver_from_dict({
        'score': rule_engine.DataType.FLOAT,
        'threshold': rule_engine.DataType.FLOAT
    }))

    # Include the threshold in the data passed to the rule engine
    data = {'score': plugin_result['score'], 'threshold': threshold}
    logger.debug(f"Rule engine data: {data}")

    # Create and evaluate the rule
    relation = rule.config.Relation
    if not relation or not relation.strip():
        logger.warning("No relation specified, defaulting to '>'")
        relation = '>'  # Default to greater than if no relation is specified
    
    # Ensure there's exactly one space between components
    rule_expression = f"score {relation.strip()} {threshold}".strip()
    logger.debug(f"Rule expression: {rule_expression}")
    logger.debug(f"Data for rule engine: {data}")
    
    try:
        rule_obj = rule_engine.Rule(rule_expression, context=context)
        match = rule_obj.matches(data)
        logger.debug(f"Rule engine result: match={match}")
        response = {"match": match, "inspection": plugin_result}
        logger.debug(f"Plugin Name: {plugin_name} API response: {response}")

        return response
    except Exception as e:
        logger.error(f"Error executing rule engine: {e}, Expression: score {relation} {threshold}")
        raise HTTPException(status_code=500, detail=f"Error executing rule engine: {str(e)}")

def main():
    # Get host and port from environment variables, with defaults
    host = os.getenv('HOST', '0.0.0.0')
    port = int(os.getenv('PORT', 8000))

    logger.info(f"Starting server on {host}:{port}")
    uvicorn.run(app, host=host, port=port)

if __name__ == "__main__":
    main()
