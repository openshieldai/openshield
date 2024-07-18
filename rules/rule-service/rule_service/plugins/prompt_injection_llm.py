import torch
from transformers import AutoTokenizer, AutoModelForSequenceClassification, pipeline

# Initialize the tokenizer and model once when the module is imported
tokenizer = AutoTokenizer.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")
model = AutoModelForSequenceClassification.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")


def handler(text: str, threshold: float) -> dict[str, bool] | bool:
    """
    Classifies the input text into a category using a pre-trained model.

    Args:
    text (str): The text to classify.
    threshold (float): The minimum score required to classify the text as a prompt injection.

    Returns:
    check_result (bool): False if the text is not a prompt injection, True otherwise.
    injection_score (float): The score of the prompt injection.


    """

    classifier = pipeline(
        "text-classification",
        model=model,
        tokenizer=tokenizer,
        truncation=True,
        max_length=512,
        device=torch.device("cuda" if torch.cuda.is_available() else "cpu"),
    )
    highest_score = 0.0
    results_all = classifier(text)
    for result in results_all:
        injection_score = round(result["score"] if result["label"] == "INJECTION" else 1 - result["score"], 2)

        if injection_score > highest_score:
            highest_score = injection_score

        if injection_score > threshold:
            print("Detected prompt injection")
            return {"check_result": True, "injection_score": highest_score}

    return {"check_result": False, "injection_score": highest_score}
