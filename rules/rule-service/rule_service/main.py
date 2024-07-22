import uvicorn
from fastapi import FastAPI, HTTPException
import rule_engine
import importlib
from pydantic import BaseModel
from typing import Dict, Any, List
class ModelConfig(BaseModel):
    LangCode: str
    ModelName: Dict[str, str]


class PIIServiceConfig(BaseModel):
    Debug: bool
    Port: int
    PIIMethod: str
    RuleBased: Dict[str, List[str]]
    NLPEngineName: str
    Models: List[ModelConfig]
    NERModelConfig: Dict[str, Dict[str, str]]


class Rule(BaseModel):
    prompt: dict
    plugin_name: str
    injection_score: float
    config: dict


app = FastAPI()


@app.post("/rule/execute")
async def execute_plugin(rule: Rule):
    try:
        plugin_module = importlib.import_module(f"plugins.{rule.plugin_name}")
    except ModuleNotFoundError:
        raise HTTPException(status_code=404, detail=f"Plugin '{rule.plugin_name}' not found")

    handler = getattr(plugin_module, 'handler')

    # Extract user message from the prompt
    user_message = next((msg['content'] for msg in rule.prompt['messages'] if msg['role'] == 'user'), None)

    if user_message is None:
        raise HTTPException(status_code=400, detail="No user message found in the prompt")

    # Call the plugin handler with all necessary parameters
    plugin_result = handler(user_message, rule.injection_score, rule.config)

    # Create a context for rule evaluation
    context = rule_engine.Context(type_resolver=rule_engine.type_resolver_from_dict({
        'check_result': rule_engine.DataType.BOOLEAN,
        'injection_score': rule_engine.DataType.FLOAT
    }))

    # Evaluate the rule
    evaluation_dict = {
        'check_result': plugin_result['check_result'],
        'injection_score': rule.injection_score
    }

    input_rule = f'injection_score > {rule.injection_score}'
    rule_obj = rule_engine.Rule(input_rule, context=context)
    match = rule_obj.matches(evaluation_dict)

    return {"match": match, "inspection": plugin_result}


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)
