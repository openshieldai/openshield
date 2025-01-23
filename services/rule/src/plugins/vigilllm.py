"""
VigillLM plugin for rule server to detect prompt injections and other LLM threats.
This plugin uses VigillLM's API endpoint to analyze prompts.
"""

import logging
import requests
import os
from typing import Dict, Any

from utils.logger_config import setup_logger

logger = setup_logger(__name__)


def get_vigilllm_url():
    url = os.getenv("VIGILLLM_API_URL")
    if not url:
        logger.error("VIGILLLM_API_URL environment variable not set")
        raise ValueError("VIGILLLM_API_URL environment variable not set")
    return url


class VigillLMAnalyzer:


    def __init__(self):
        self.api_url = get_vigilllm_url().rstrip('/')
        self.analyze_endpoint = f"{self.api_url}/analyze/prompt"
        logger.info(f"Initialized VigillLM analyzer with API URL: {self.api_url}")

    def analyze_prompt(self, text: str) -> Dict[str, Any]:

        try:
            logger.debug(f"Sending prompt to VigillLM for analysis: {text[:100]}...")

            response = requests.post(
                self.analyze_endpoint,
                json={"prompt": text},
                timeout=30
            )

            response.raise_for_status()
            result = response.json()

            logger.debug(f"Received VigillLM analysis result: {result}")
            return result

        except requests.exceptions.RequestException as e:
            error_msg = f"Error calling VigillLM API: {str(e)}"
            logger.error(error_msg)
            raise RuntimeError(error_msg)



analyzer = None


def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:

    global analyzer

    try:
        if analyzer is None:
            analyzer = VigillLMAnalyzer()
            logger.info("Initialized VigillLM analyzer")


        result = analyzer.analyze_prompt(text)


        scanners = result.get('results', {})
        risk_score = 0.0

        if 'scanner:transformer' in scanners:
            transformer_matches = scanners['scanner:transformer'].get('matches', [])
            if transformer_matches and len(transformer_matches) > 0:
                # Use the first transformer match score
                risk_score = transformer_matches[0]['score']

        return {
            "check_result": risk_score > threshold,
            "score": risk_score,
            "details": {
                "messages": result.get('messages', []),
                "scanners": result.get('results', {}),
                "uuid": result.get('uuid'),
                "timestamp": result.get('timestamp')
            }
        }

    except Exception as e:
        logger.error(f"Error in VigillLM analysis: {str(e)}")
        return {
            "check_result": False,
            "score": 0.0,
            "details": {"error": str(e)}
        }