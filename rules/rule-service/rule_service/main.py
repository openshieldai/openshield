import uvicorn
from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel
from typing import List, Optional
import importlib
import rule_engine
import logging

# Configure logging
logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger(__name__)


class Message(BaseModel):
    role: str
    content: str


class Prompt(BaseModel):
    model: Optional[str] = None
    messages: Optional[List[Message]] = None
    role: Optional[str] = None
    content: Optional[str] = None


class Config(BaseModel):
    PluginName: str
    Threshold: float

    # Allow any additional fields
    class Config:
        extra = "allow"


class Rule(BaseModel):
    prompt: Prompt
    config: Config


app = FastAPI()


@app.post("/rule/execute")
async def execute_plugin(rule: Rule):
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

    if rule.prompt.messages:
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
    rule_obj = rule_engine.Rule('score > threshold', context=context)
    match = rule_obj.matches(data)
    logger.debug(f"Rule engine result: match={match}")

    response = {"match": match, "inspection": plugin_result}
    logger.debug(f"Plugin Name: {plugin_name} API response: {response}")

    return response


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)