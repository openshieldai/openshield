import requests
from typing import Dict, Any


def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:
    api_key = config.get('ApiKey')
    url = config.get('Url')

    if not url:
        raise ValueError("English detection URL not set in configuration")

    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json"
    }
    payload = {"inputs": text}

    response = requests.post(url, headers=headers, json=payload)
    response.raise_for_status()

    results = response.json()

    if not results or not isinstance(results[0], list):
        raise ValueError("Unexpected response format")

    english_score = next((score['score'] for score in results[0] if score['label'] == 'en'), 0)

    return {
        "check_result": english_score > threshold,
        "score": english_score,
    }
