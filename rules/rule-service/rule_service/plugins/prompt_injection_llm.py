import torch
from transformers import AutoTokenizer, AutoModelForSequenceClassification, pipeline

# Initialize the tokenizer and model once when the module is imported
tokenizer = AutoTokenizer.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")
model = AutoModelForSequenceClassification.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")


def handler(text: str, threshold: float, config: dict) -> dict:
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
        "injection_score": injection_score
    }