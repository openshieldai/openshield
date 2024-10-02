import torch
from transformers import AutoTokenizer, AutoModelForSequenceClassification
from typing import Dict, Any


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