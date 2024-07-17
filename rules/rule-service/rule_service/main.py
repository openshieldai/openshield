from fastapi import FastAPI
from plugins.prompt_injection_llm import classify_text

app = FastAPI()


@app.get("/plugins/{plugin_name}")
async def read_plugin(plugin_name: str):
    something = classify_text("something")
    print(something)
    return {"plugin_name": plugin_name, "something": something}


# @app.get("/items/{item_id}")
# async def read_item(item_id: int, q: Union[str, None] = None):
#     return {"item_id": item_id, "q": q}