"""
PromptGuard plugin for detecting prompt injection attacks using the PromptGuard model.
"""

import os
import logging
from typing import Dict, Any
import torch
from transformers import AutoTokenizer, AutoModelForSequenceClassification
from huggingface_hub import login, HfApi

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def get_huggingface_token():
    """Get token from environment with proper error handling."""
    token = os.getenv("HUGGINGFACE_TOKEN") or os.getenv("HUGGINGFACE_API_KEY")
    if not token:
        logger.error("HUGGINGFACE_TOKEN or HUGGINGFACE_API_KEY environment variable not set")
        return None
    return token

class PromptGuardAnalyzer:
    def __init__(self):
        self.token = get_huggingface_token()
        if not self.token:
            raise ValueError("HuggingFace token not set")

        try:
            login(token=self.token, write_permission=True)
            self.api = HfApi()
            logger.info("Successfully logged into HuggingFace")
        except Exception as e:
            logger.error(f"HuggingFace authentication error: {str(e)}")
            raise

        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        logger.info(f"Using device: {self.device}")

        try:
            logger.info("Loading PromptGuard model...")
            self.model = AutoModelForSequenceClassification.from_pretrained(
                "meta-llama/Prompt-Guard-86M",
                use_auth_token=self.token,
                trust_remote_code=True
            )
            self.tokenizer = AutoTokenizer.from_pretrained(
                "meta-llama/Prompt-Guard-86M",
                use_auth_token=self.token,
                trust_remote_code=True
            )
            self.model.to(self.device)
            self.model.eval()
            logger.info("PromptGuard model loaded successfully")
        except Exception as e:
            logger.error(f"Error loading PromptGuard model: {str(e)}")
            raise

    def get_class_probabilities(self, text: str, temperature: float = 3.0) -> torch.Tensor:
        inputs = self.tokenizer(
            text,
            return_tensors="pt",
            padding=True,
            truncation=True,
            max_length=512
        ).to(self.device)

        with torch.no_grad():
            logits = self.model(**inputs).logits
            scaled_logits = logits / temperature
            probabilities = torch.nn.functional.softmax(scaled_logits, dim=-1)

        return probabilities[0]

    def analyze_text(self, text: str, temperature: float = 3.0) -> Dict[str, Any]:
        try:
            probabilities = self.get_class_probabilities(text, temperature)

            scores = {
                "benign_probability": probabilities[0].item(),
                "injection_probability": probabilities[1].item(),
                "jailbreak_probability": probabilities[2].item()
            }

            if scores["jailbreak_probability"] > scores["injection_probability"]:
                risk_score = scores["jailbreak_probability"]
                classification = "jailbreak"
            else:
                risk_score = scores["injection_probability"]
                classification = "injection"

            logger.info(f"\nAnalyzing text: {text[:100]}...")
            logger.info(f"Probabilities: {scores}")
            logger.info(f"Classification: {classification}")

            return {
                "score": risk_score,
                "details": scores,
                "classification": classification
            }

        except Exception as e:
            logger.error(f"Error during analysis: {e}")
            raise

# Initialize global analyzer
analyzer = None
try:
    logger.info("Initializing PromptGuard analyzer...")
    analyzer = PromptGuardAnalyzer()
    logger.info("PromptGuard analyzer initialized successfully")
except Exception as e:
    logger.error(f"Failed to initialize PromptGuard analyzer: {str(e)}")

def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Analyzes text for prompt injection using PromptGuard.

    Args:
        text: The text to analyze
        threshold: Score threshold for detecting injection
        config: Configuration parameters including temperature

    Returns:
        dict containing:
            - check_result: bool if score exceeds threshold
            - score: risk score from 0-1
            - details: probabilities and classification details
    """
    try:
        if analyzer is None:
            raise RuntimeError("PromptGuard analyzer not initialized")

        temperature = config.get('temperature', 3.0)
        results = analyzer.analyze_text(text, temperature)

        return {
            "check_result": results["score"] > threshold,
            "score": results["score"],
            "details": {
                "benign_probability": results["details"]["benign_probability"],
                "injection_probability": results["details"]["injection_probability"],
                "jailbreak_probability": results["details"]["jailbreak_probability"],
                "classification": results["classification"]
            }
        }

    except Exception as e:
        logger.error(f"Error in PromptGuard analysis: {str(e)}")
        return {
            "check_result": False,
            "score": 0.0,
            "details": {"error": str(e)}
        }