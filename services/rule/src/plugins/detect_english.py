"""
This module provides a handler for detecting English language content using a pre-trained language detection model.

The `LanguageDetector` class initializes the model and tokenizer from the Hugging Face Transformers library.
It uses the `xlm-roberta-base-language-detection` model to detect the language of a given text.

The `handler` function uses the `LanguageDetector` to check if the text is in English and returns a result indicating the presence of English content along with a score.

Classes:
- LanguageDetector: Initializes the language detection model and provides a method to detect the language of a given text.

Functions:
- handler: Checks the text for English language content using the `LanguageDetector` and returns the result.

Dependencies:
- torch: PyTorch library for tensor computations.
- transformers: Hugging Face Transformers library for pre-trained models.
"""

import torch
from transformers import AutoTokenizer, AutoModelForSequenceClassification
from typing import Dict, Any
from utils.logger_config import setup_logger
logger = setup_logger(__name__)


class LanguageDetector:
    def __init__(self, model_name="papluca/xlm-roberta-base-language-detection"):
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.tokenizer = AutoTokenizer.from_pretrained(model_name)
        self.model = AutoModelForSequenceClassification.from_pretrained(model_name).to(self.device)

    def detect(self, text: str) -> Dict[str, float]:
        inputs = self.tokenizer(text, return_tensors="pt", truncation=True, max_length=512).to(self.device)
        with torch.no_grad():
            outputs = self.model(**inputs)

        scores = torch.nn.functional.softmax(outputs.logits, dim=-1)
        predictions = {
            self.model.config.id2label[i]: score.item()
            for i, score in enumerate(scores[0])
        }
        return predictions


def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:
    detector = LanguageDetector()
    results = detector.detect(text)

    english_score = results.get('en', 0)

    return {
        "check_result": english_score > threshold,
        "score": english_score,
    }
