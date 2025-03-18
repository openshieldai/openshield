"""
This module provides a handler for detecting code content using a pre-trained programming language identification model.

The `CodeDetector` class initializes the model and tokenizer from the Hugging Face Transformers library.
It uses the `philomath-1209/programming-language-identification` model to detect the programming language of a given input text.

The `handler` function uses the `CodeDetector` to check whether the text contains code content and returns a result indicating the presence of code content along with a score.

Classes:
- CodeDetector: Initializes the programming language identification model and provides a method to detect the programming language of a given text.

Functions:
- handler: Checks the text for code content using the `CodeDetector` and returns the result.

Dependencies:
- torch: PyTorch library for tensor computations.
- transformers: Hugging Face Transformers library for pre-trained models and tokenizers.
"""

import torch
import os
from transformers import AutoTokenizer, AutoModelForSequenceClassification
from typing import Dict
from utils.logger_config import setup_logger

logger = setup_logger(__name__, os.getenv('LOG_REMOTE', False))

class CodeDetector:
    def __init__(self, model="philomath-1209/programming-language-identification"):
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        logger.info(f"Using device: {self.device}")

        self.tokenizer = AutoTokenizer.from_pretrained(model)
        self.model = AutoModelForSequenceClassification.from_pretrained(model).to(self.device)

    def detect(self, text: str) -> Dict[str, float]:
        inputs = self.tokenizer(text, return_tensors="pt", truncation=True).to(self.device)

        with torch.no_grad():
            logits = self.model(**inputs).logits
            result = torch.nn.functional.softmax(logits, dim=-1)

        predictions = {
            self.model.config.id2label[i]: score.item()
            for i, score in enumerate(result[0])
        }

        return predictions


def handler(text: str, threshold: float, config: Dict[str, float]) -> Dict[str, float]:
    try:
        detector = CodeDetector()
        results = detector.detect(text)
        score = max(results.values())

        language, score = max(results.items(), key=lambda x: x[1])
        logger.info(f"Detected language: {language}, Score: {score}")

        return {"check_result": score > threshold, "score": score}

    except Exception as e:
        logger.error(f"Error executing code detection: {e}")
        return {"check_result": False, "score": 0.0, "details" : {"error": str(e)}}
