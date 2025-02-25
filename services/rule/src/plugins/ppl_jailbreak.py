"""
This modules provides a handler for detecting jailbreak attempts using heuristic rules based on perplexity (ppl) scores.

The `PerplexityJailbreakDetector` class initializes a GPT-2 model and tokenizer from the Hugging Face Transformers library.
It provides methods to calculate the perplexity of a given input string and detect jailbreak attempts based on perplexity scores.

The `handler` function uses the `PerplexityJailbreakDetector` to check for jailbreak attempts in the input text and returns a result indicating the presence of jailbreak content along with a score.

Classes:
- PerplexityJailbreakDetector: Initializes the GPT-2 model and tokenizer and provides methods to calculate perplexity and detect jailbreak attempts.

Functions:
- parse_ppl_config: Extracts and returns relevant perplexity configurations from the input config.
- handler: Checks the text for jailbreak attempts using the `PerplexityJailbreakDetector` and returns the result.

Dependencies:
- torch: PyTorch library for tensor computations.
- transformers: Hugging Face Transformers library for pre-trained models.
"""

import torch
from transformers import GPT2LMHeadModel, GPT2TokenizerFast
from typing import Dict, Any
from utils.logger_config import setup_logger
logger = setup_logger(__name__)

class PerplexityJailbreakDetector:
    """Class for detecting jailbreak attempts using perplexity scores."""
    def __init__(self, model_name="openai-community/gpt2-medium"):
        """Initialize the GPT-2 model and tokenizer."""
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.tokenizer = GPT2TokenizerFast.from_pretrained(model_name, clean_up_tokenization_spaces=False)
        self.model = GPT2LMHeadModel.from_pretrained(model_name).to(self.device)

    def get_perplexity(self, text: str) -> float:
        """
        Calculate the perplexity of the input text using a sliding window strategy.

        Args:
            text (str): The input text to calculate perplexity for.
        Returns:
            float: The perplexity score of the input text.
        """
        encodings = self.tokenizer(text, return_tensors="pt")

        max_length = self.model.config.n_positions
        stride = 512
        seq_len = encodings.input_ids.size(1)

        nlls = list()
        prev_end_loc = 0
        for begin_loc in range(0, seq_len, stride):
            end_loc = min(begin_loc + max_length, seq_len)
            trg_len = end_loc - prev_end_loc 
            input_ids = encodings.input_ids[:, begin_loc:end_loc].to(self.device)
            target_ids = input_ids.clone()
            target_ids[:, :-trg_len] = -100

            with torch.no_grad():
                outputs = self.model(input_ids, labels=target_ids)
                neg_log_likelihood = outputs.loss

            nlls.append(neg_log_likelihood)

            prev_end_loc = end_loc
            if end_loc == seq_len:
                break
        
        perplexity = torch.exp(torch.mean(torch.stack(nlls)))
        return perplexity.cpu().detach().numpy().item()
    
    def detect_jailbreak_length_per_perplexity(self, text: str, threshold: float) -> bool:
        """
        Detect jailbreak attempts based on the length of the text and its perplexity score.

        Args:
            text (str): The input text to analyze.
            threshold (float): The threshold perplexity score for jailbreak detection.
        Returns:
            bool: True if jailbreak attempt is detected, False otherwise.
        """
        ppl = self.get_perplexity(text)
        score = len(text) / ppl
        result = score > threshold
        logger.info(f"Jailbreak detection result: {result}, Score: {score:.3f}")
        return result
    
    def detect_jailbreak_prefix_suffix_perplexity(self, text: str, threshold: float) -> bool:
        """
        Detect jailbreak attempts based on the perplexity scores of the prefix and suffix of the text.
        
        Args:
            text (str): The input text to analyze.
            threshold (float): The threshold perplexity score for jailbreak detection.
        Returns:
            bool: True if jailbreak attempt is detected, False otherwise.
        """
        words = text.strip().split()
        
        if len(words) < 20:
            logger.info("Text length is less than 20 words, skipping prefix-suffix perplexity check.")
            return False
        
        prefix = " ".join(words[:20])
        suffix = " ".join(words[-20:])

        prefix_ppl = self.get_perplexity(prefix)
        suffix_ppl = self.get_perplexity(suffix)

        prefix_result = prefix_ppl > threshold
        suffix_result = suffix_ppl > threshold
        logger.info(f"Prefix result: {prefix_result}, Prefix perplexity: {prefix_ppl:.3f}")
        logger.info(f"Suffix result: {suffix_result}, Suffix perplexity: {suffix_ppl:.3f}")

        return prefix_result or suffix_result

def parse_ppl_config(config: Dict[str, Any]) -> Dict[str, Any]:
    """Extracts and returns relevant perplexity configurations."""
    ppl_service_config = config.get("PPLService", {})
    
    return {
        "length_per_ppl_enabled": ppl_service_config.get("length_per_perplexity", {}).get("enabled", False),
        "length_per_ppl_threshold": ppl_service_config.get("length_per_perplexity", {}).get("threshold", 140.55),
        "prefix_suffix_ppl_enabled": ppl_service_config.get("prefix_suffix_perplexity", {}).get("enabled", False),
        "prefix_suffix_ppl_threshold": ppl_service_config.get("prefix_suffix_perplexity", {}).get("threshold", 1845.65),
    }

def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:
    """FastAPI compatible handler function for jailbreak detection."""
    try:
        ppl_config = parse_ppl_config(config)
        logger.info(f"Extracted configuration: {ppl_config}")

        detector = PerplexityJailbreakDetector()
        
        if ppl_config["prefix_suffix_ppl_enabled"]:
            prefix_suffix_ppl_result = detector.detect_jailbreak_prefix_suffix_perplexity(text, ppl_config["prefix_suffix_ppl_threshold"])
        
        if prefix_suffix_ppl_result:
            return {
                "check_result": True,
                "score": 1.0,
            }

        if ppl_config["length_per_ppl_enabled"]:
            length_per_ppl_result = detector.detect_jailbreak_length_per_perplexity(text, ppl_config["length_per_ppl_threshold"])

        if length_per_ppl_result:
            return {
                "check_result": True,
                "score": 1.0,
            }
        
        return {
            "check_result": False,
            "score": 0.0,
        }
    except Exception as e:
        logger.error(f"Error in perplexity jailbreak detection: {e}")
        return {
            "check_result": False,
            "score": 0.0,
        }