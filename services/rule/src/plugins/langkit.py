"""
This module provides a LangKit plugin for the ruleserver to analyze text using the LangKit injection detection.
"""

import logging
import whylogs as why
from langkit import injections, extract
from utils.logger_config import setup_logger

logger = setup_logger(__name__)

def initialize_langkit():

    try:
        schema = injections.init()
        logger.info("LangKit schema initialized successfully")
        return schema
    except Exception as e:
        logger.error(f"Failed to initialize LangKit schema: {e}")
        raise

def analyze_text(text: str, schema) -> dict:

    try:

        result = extract({"prompt": text}, schema=schema)


        injection_score = result.get("prompt.injection", 0.0)  # Similarity to known attacks

        scores = {
            "injection_detection": {
                "score": injection_score,
                "description": "Similarity to known injection/jailbreak attacks"
            }
        }

        logger.info(f"Scores: {scores}")
        return scores

    except Exception as e:
        logger.error(f"Error analyzing text with LangKit: {e}")
        raise


schema = initialize_langkit()

def handler(text: str, threshold: float, config: dict) -> dict:

    try:
        logger.info(f"Analyzing text with LangKit injection detection. Threshold: {threshold}")

        scores = analyze_text(text, schema)

        # Check which scores exceed threshold
        blocked_reasons = {}
        for check_name, result in scores.items():
            if result["score"] > threshold:
                blocked_reasons[check_name] = {
                    "score": result["score"],
                    "description": result["description"]
                }

        is_blocked = len(blocked_reasons) > 0
        max_score = max([result["score"] for result in scores.values()])

        logger.info(f"Analysis complete. Blocked: {is_blocked}")
        if blocked_reasons:
            logger.info(f"Blocking reasons: {blocked_reasons}")

        return {
            "check_result": is_blocked,
            "score": max_score,
            "details": {
                "all_scores": scores,
                "threshold": threshold,
                "blocked_reasons": blocked_reasons,
                "is_blocked": is_blocked
            }
        }

    except Exception as e:
        logger.error(f"Error in LangKit handler: {str(e)}")
        return {
            "check_result": False,
            "score": 0.0,
            "details": {"error": str(e)}
        }