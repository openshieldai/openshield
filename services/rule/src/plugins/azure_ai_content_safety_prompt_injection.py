"""
This module provides a handler for detecting prompt injection attacks using Azure Content Safety API.

The handler function `handler` takes a text input and checks for any prompt injection attacks by calling the Azure Content Safety API.
If an attack is detected, it returns a result indicating the presence of the attack along with a score.

Environment variables required:
- CONTENT_SAFETY_KEY: The subscription key for the Azure Content Safety API.
- CONTENT_SAFETY_ENDPOINT: The endpoint URL for the Azure Content Safety API.

Functions:
- content_safety_api: Makes a request to the Azure Content Safety API with the provided text.
- handler: Checks the text for prompt injection attacks using the Azure Content Safety API and returns the result.
"""


from typing import Dict, Any
import os
import requests

from srsly import json_dumps
from utils.logger_config import setup_logger
logger = setup_logger(__name__, os.getenv('LOG_REMOTE', False))

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
