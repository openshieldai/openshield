import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional
import importlib
import rule_engine
import logging

logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger(__name__)


class Message(BaseModel):
    role: str
    content: str


class Prompt(BaseModel):
    model: Optional[str]
    messages: List[Message]


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
        plugin_name = rule.config.PluginName.lower()
        plugin_module = importlib.import_module(f"plugins.{plugin_name}")
    except ModuleNotFoundError:
        raise HTTPException(status_code=404, detail=f"Plugin '{plugin_name}' not found")

    handler = getattr(plugin_module, 'handler')

    user_message = next((msg.content for msg in rule.prompt.messages if msg.role == 'user'), None)
    if user_message is None:
        raise HTTPException(status_code=400, detail="No user message found in the prompt")

    threshold = rule.config.Threshold
    plugin_result = handler(user_message, threshold, rule.config.model_dump())

    logger.debug(f"Plugin result: {plugin_result}")

    if not isinstance(plugin_result, dict) or 'score' not in plugin_result:
        raise HTTPException(status_code=500, detail="Invalid plugin result format")

    # Set up context for rule engine
    context = rule_engine.Context(type_resolver=rule_engine.type_resolver_from_dict({
        'score': rule_engine.DataType.FLOAT,
        'threshold': rule_engine.DataType.FLOAT
    }))

    # Include the threshold in the data passed to the rule engine
    data = {'score': plugin_result['score'], 'threshold': threshold}

    # Create and evaluate the rule
    rule_obj = rule_engine.Rule('score > threshold', context=context)
    match = rule_obj.matches(data)

    logger.debug(f"Rule engine result: match={match}")
    logger.debug(f"Final data being returned: match={match}, inspection={plugin_result}")

    response = {"match": match, "inspection": plugin_result}
    logger.debug(f"API response: {response}")

    return response


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)
