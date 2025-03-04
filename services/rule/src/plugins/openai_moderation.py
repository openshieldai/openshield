"""
This module provides a handler for detecting content moderation issues using the OpenAI Moderation API.

The `handler` function takes a text input and checks for any content that violates moderation policies by calling the OpenAI Moderation API.
If a violation is detected, it returns a result indicating the presence of the violation along with a score.

Environment variables required:
- OPENAI_API_KEY: The API key for the OpenAI Moderation API.

Functions:
- handler: Checks the text for content moderation issues using the OpenAI Moderation API and returns the result.

Dependencies:
- openai: OpenAI library for accessing the Moderation API.
- os: Provides a way to access environment variables.
"""

import os
from openai import OpenAI
from utils.logger_config import setup_logger
logger = setup_logger(__name__)

client = OpenAI(
    api_key=os.environ.get("OPENAI_API_KEY"),
)


def handler(text: str, threshold: float, _: dict) -> dict:
    moderation_result = client.moderations.create(
        model="omni-moderation-latest",
        input=text
    )


    for result in moderation_result.results:
        for key, value in result.categories:
            if value:
                return {
                    "check_result": 1 > threshold,
                    "score": 1
                }


    return {
        "check_result": 0 > threshold,
        "score": 0
    }
