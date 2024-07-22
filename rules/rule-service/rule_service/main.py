import uvicorn
from fastapi import FastAPI, HTTPException
import rule_engine
import importlib
from pydantic import BaseModel


class Rule(BaseModel):
    prompt: dict
    plugin_name: str
    injection_score: float


app = FastAPI()


@app.post("/rule/execute")
async def read_plugin(rule: Rule):
    try:
        plugin_module = importlib.import_module(f"plugins.{rule.plugin_name}")
    except ModuleNotFoundError:
        raise HTTPException(status_code=404, detail=f"Plugin '{rule.plugin_name}' not found")
    handler = getattr(plugin_module, 'handler')
    plugin_result = None
    for attrs in rule.prompt['messages']:
        if attrs['role'] == 'user':
            plugin_result = handler(attrs['content'], rule.injection_score)
            break
        else:
            continue

    if plugin_result is None:
        raise HTTPException(status_code=400, detail="No user message found in the prompt")

    # Create a context that includes both check_result and injection_score
    context = rule_engine.Context(type_resolver=rule_engine.type_resolver_from_dict({
        'check_result': rule_engine.DataType.BOOLEAN,
        'injection_score': rule_engine.DataType.FLOAT
    }))

    # Create a dictionary with both values
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