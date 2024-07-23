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

    # Extract user message from the prompt
    user_message = next((msg.content for msg in rule.prompt.messages if msg.role == 'user'), None)

    if user_message is None:
        raise HTTPException(status_code=400, detail="No user message found in the prompt")

    # Call the plugin handler with all necessary parameters
    threshold = rule.config.Threshold
    plugin_result = handler(user_message, threshold, rule.config.dict())

    # Create a context for rule evaluation
    context = rule_engine.Context(type_resolver=rule_engine.type_resolver_from_dict({
        'check_result': rule_engine.DataType.BOOLEAN,
        'injection_score': rule_engine.DataType.FLOAT
    }))

    # Ensure plugin_result is a dictionary
    if not isinstance(plugin_result, dict):
        raise HTTPException(status_code=500, detail="Plugin result must be a dictionary")

    # Ensure check_result is present in plugin_result
    if 'check_result' not in plugin_result:
        raise HTTPException(status_code=500, detail="Plugin result must contain 'check_result'")

    # Construct the rule based on the threshold and any numeric fields in the result
    rule_parts = [f"{k} > {threshold}" for k in plugin_result if isinstance(plugin_result[k], (int, float)) and k != 'check_result']
    rule_str = " or ".join(rule_parts) if rule_parts else "check_result == True"

    logger.debug(f"Constructed rule string: {rule_str}")

    # Evaluate the rule
    try:
        rule_obj = rule_engine.Rule(rule_str, context=context)
        match = rule_obj.matches(plugin_result)
    except rule_engine.errors.SymbolResolutionError as e:
        logger.error(f"Error evaluating rule: {str(e)}")
        match = plugin_result.get('check_result', False)

    return {"match": match, "inspection": plugin_result}
if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)