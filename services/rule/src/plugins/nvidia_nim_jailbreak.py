"""
NVIDIA NIM Jailbreak Detection Plugin

This plugin interfaces with NVIDIA's NIM Jailbreak Detection API

Environment Variables:
- NVIDIA_API_KEY: Required API key for authentication

"""

import os
import requests
from typing import Dict, Any
from utils.logger_config import setup_logger

logger = setup_logger(__name__, os.getenv('LOG_REMOTE', False))


def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Processes text through NVIDIA NIM Jailbreak Detection API

    Args:
        text: User input text to analyze
        threshold: e threshold (unused in this plugin)
        config:  configuration

    Returns:
        Dict with 'score' (float) and 'check_result' (bool) detection results

    Raises:
        ValueError: For missing API key or invalid API responses
        RuntimeError: For API request failures
    """
    # Get API key from environment
    api_key = os.getenv("NVIDIA_API_KEY")
    if not api_key:
        logger.error("Missing NVIDIA_API_KEY environment variable")
        raise ValueError("NVIDIA_API_KEY environment variable required")


    url = "https://ai.api.nvidia.com/v1/security/nvidia/nemoguard-jailbreak-detect"
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {api_key}"
    }
    payload = {"input": text}

    try:
        response = requests.post(url, headers=headers, json=payload)
        response.raise_for_status()
    except requests.exceptions.RequestException as e:
        logger.error(f"NVIDIA API request failed: {str(e)}")
        raise RuntimeError(f"NVIDIA API request failed: {str(e)}")

    try:
        response_data = response.json()
        score = float(response_data["score"])
        jailbreak = response_data["jailbreak"]
    except (KeyError, ValueError) as e:
        logger.error(f"Invalid API response format: {str(e)}")
        raise ValueError(f"Invalid API response format: {str(e)}")

    return {
        "check_result": jailbreak,
        "score": score,
    }
