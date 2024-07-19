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
    check_result = None
    for attrs in rule.prompt['messages']:
        if attrs['role'] == 'user':
            check_result = handler(attrs['content'], rule.injection_score)
            break
        else:
            continue
    context = rule_engine.Context(type_resolver=rule_engine.type_resolver_from_dict({
        'check_result': rule_engine.DataType.BOOLEAN,
        'injection_score': rule_engine.DataType.FLOAT
    }))
    input_rule = f'injection_score > {rule.injection_score}'

    rule = rule_engine.Rule(input_rule, context=context)
    match = rule.matches(check_result)
    return {"match": match, "inspection": check_result}
