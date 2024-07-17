from transformers import AutoTokenizer, AutoModelForSequenceClassification

# Initialize the tokenizer and model once when the module is imported
tokenizer = AutoTokenizer.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")
model = AutoModelForSequenceClassification.from_pretrained("protectai/deberta-v3-base-prompt-injection-v2")

def classify_text(text: str) -> str:
    """
    Classifies the input text into a category using a pre-trained model.

    Args:
    text (str): The text to classify.

    Returns:
    str: The predicted category label.
    """
    inputs = tokenizer(text, return_tensors="pt")
    outputs = model(**inputs)
    logits = outputs.logits
    predicted_class_idx = logits.argmax().item()
    return model.config.id2label[predicted_class_idx]
