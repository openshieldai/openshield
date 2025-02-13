import importlib
import json
import logging
import os
from typing import List, Optional

import rule_engine
import uvicorn
from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel

from utils.logger_config import setup_logger

logger = setup_logger(__name__)


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
    logger.info(f"Received rule: {rule.model_dump_json()}")
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


from typing import Optional


class ScanRule(BaseModel):
    name: str
    type: str
    enabled: bool
    order_number: int
    config: dict
    action: dict
    threshold: Optional[float] = None


class ScanRequest(BaseModel):
    input: str
    rules: List[ScanRule]


class SimplifiedRuleResult(BaseModel):
    rule_type: str
    status: str


class ScanResponse(BaseModel):
    blocked: bool
    rule_results: List[SimplifiedRuleResult]


@app.post("/scan", response_model=ScanResponse)
async def scan(scan_request: ScanRequest):
    """
    Scan endpoint that mimics the Go InputCheck function.
    It reads the 'input' string and a list of rules (with full config) from the POST request,
    builds a Rule for each using the provided configuration, and calls execute_plugin for each.
    """
    user_input = scan_request.input
    rules_list = scan_request.rules
    overall_blocked = False
    results = []

    # Sort rules by order_number
    sorted_rules = sorted(rules_list, key=lambda r: r.order_number)

    for rule in sorted_rules:
        # Skip disabled rules
        if not rule.enabled:
            results.append(SimplifiedRuleResult(
                rule_type=rule.config.get("plugin_name", rule.name),
                status="skipped"
            ))
            continue

        # Prepare config data by copying and mapping keys as needed.
        config_data = rule.config.copy()
        # Map 'plugin_name' (from YAML) to 'PluginName' (expected by our Config model)
        if "plugin_name" in config_data:
            config_data["PluginName"] = config_data.pop("plugin_name")
        # Set default Relation if not present
        if "Relation" not in config_data:
            config_data["Relation"] = ">="
        # Override Threshold if provided at the rule level.
        if rule.threshold is not None:
            config_data["Threshold"] = rule.threshold
        # Ensure a Threshold exists; if not, set a default (e.g., 0.5)
        if "Threshold" not in config_data:
            config_data["Threshold"] = 0.5

        # Build the Rule object expected by execute_plugin.
        rule_obj = Rule(
            prompt=Prompt(role="user", content=user_input),
            config=Config(**config_data)
        )

        try:
            plugin_result = await execute_plugin(rule_obj)
        except Exception as e:
            logger.error(f"Error executing rule for {config_data.get('PluginName', rule.name)}: {e}")
            results.append(SimplifiedRuleResult(
                rule_type=config_data.get("PluginName", rule.name),
                status="matched"
            ))
            overall_blocked = True
            continue

        rule_match = plugin_result.get("match", False)
        status = "passed"
        # If the rule's action is "block" and the plugin indicates a match, mark as matched.
        if rule.action.get("type") == "block" and rule_match:
            status = "matched"
            overall_blocked = True

        results.append(SimplifiedRuleResult(
            rule_type=config_data.get("PluginName", rule.name),
            status=status
        ))

    return ScanResponse(blocked=overall_blocked, rule_results=results)
def main():
    # Get host and port from environment variables, with defaults
    host = os.getenv('HOST', '0.0.0.0')
    port = int(os.getenv('PORT', 8000))

    logger.info(f"Starting server on {host}:{port}")
    uvicorn.run(app, host=host, port=port)


if __name__ == "__main__":
    main()
