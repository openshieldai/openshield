import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Dict, Optional
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
        logger.debug(f"Received rule: {rule}")
        plugin_name = rule.config.PluginName.lower()
        plugin_module = importlib.import_module(f"plugins.{plugin_name}")
    except ModuleNotFoundError:
        raise HTTPException(status_code=404, detail=f"Plugin '{plugin_name}' not found")

    handler = getattr(plugin_module, 'handler')

    user_message = next((msg.content for msg in rule.prompt.messages if msg.role == 'user'), None)
    if user_message is None:
        raise HTTPException(status_code=400, detail="No user message found in the prompt")

    threshold = rule.config.Threshold
    plugin_result = handler(user_message, threshold, rule.config.dict())

    if not isinstance(plugin_result, dict) or 'check_result' not in plugin_result:
        raise HTTPException(status_code=500, detail="Invalid plugin result format")

    return {"match": plugin_result['check_result'], "inspection": plugin_result}
if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)