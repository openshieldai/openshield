"""
This module provides a handler for detecting prompt injection attacks using a pre-trained language model.

The `handler` function uses the `transformers` library to classify text and determine if it contains prompt injection attacks.
It initializes the tokenizer and model from the `protectai/deberta-v3-base-prompt-injection-v2` model.

Classes:
- None

Functions:
- handler: Uses the pre-trained model to classify text and detect prompt injection attacks.

Dependencies:
- torch: PyTorch library for tensor computations.
- transformers: Hugging Face Transformers library for pre-trained models.
"""

import torch
import os
from transformers import AutoTokenizer, AutoModelForSequenceClassification, pipeline

from utils.logger_config import setup_logger
logger = setup_logger(__name__, os.getenv('LOG_REMOTE', False))

# Initialize the tokenizer and model once when the module is imported
tokenizer = AutoTokenizer.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")
model = AutoModelForSequenceClassification.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")


def handler(text: str, threshold: float, config: dict) -> dict:
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    logger.info(f"Using device: {device}")

    classifier = pipeline(
        "text-classification",
        model=model,
        tokenizer=tokenizer,
        truncation=True,
        max_length=512,
        device=torch.device("cuda" if torch.cuda.is_available() else "cpu"),
    )

    results = classifier(text)
    injection_score = round(results[0]["score"] if results[0]["label"] == "INJECTION" else 1 - results[0]["score"], 2)

    return {
        "check_result": injection_score > threshold,
        "score": injection_score
    }
