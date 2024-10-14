from typing import Dict, Any
import os
import requests

from srsly import json_dumps

key = os.environ["CONTENT_SAFETY_KEY"]
if not key:
    raise ValueError("CONTENT_SAFETY_KEY is not set")

endpoint = os.environ["CONTENT_SAFETY_ENDPOINT"]
if not endpoint:
    raise ValueError("CONTENT_SAFETY_ENDPOINT is not set")



def content_safety_api(text: str):
    url = f'{endpoint}/contentsafety/text:shieldPrompt?api-version=2024-09-01'
    headers = {
        'Ocp-Apim-Subscription-Key': key,
        'Content-Type': 'application/json'
    }
    payload = {
        "userPrompt": text
    }
    response = requests.post(url, headers=headers, data=json_dumps(payload))
    return response.json()


def handler(text: str, _threshold: float, _config: Dict[str, Any]) -> dict:
    content_safety_result = content_safety_api(text)
    print("Azure Content Safety result: %s", content_safety_result["userPromptAnalysis"]["attackDetected"])
    if content_safety_result["userPromptAnalysis"]["attackDetected"]:
        return dict(check_result=True, score=1)
    return dict(check_result=False, score=0)