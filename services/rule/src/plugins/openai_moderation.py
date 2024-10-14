import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ.get("OPENAI_API_KEY"),
)


def handler(text: str, threshold: float, _: dict) -> dict:
    moderation_result = client.moderations.create(
        model="omni-moderation-latest",
        input=text
    )


    for result in moderation_result.results:
        print(result.categories)
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