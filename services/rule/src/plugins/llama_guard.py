"""
LlamaGuard plugin for content safety analysis using Meta's Llama Guard 3-1B model.
"""

import os
import logging
from typing import Dict, Any, List, Optional
import torch
import accelerate
from transformers import AutoModelForCausalLM, AutoTokenizer
from huggingface_hub import login, HfApi

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)


def get_huggingface_token():
    token = os.getenv("HUGGINGFACE_TOKEN") or os.getenv("HUGGINGFACE_API_KEY")
    if not token:
        logger.error("HUGGINGFACE_TOKEN or HUGGINGFACE_API_KEY environment variable not set")
        return None
    return token


class LlamaGuardAnalyzer:
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
            logger.info("Loading Llama Guard model...")
            model_id = "meta-llama/Llama-Guard-3-1B"

            if torch.cuda.is_available():
                logger.info("Using GPU for model loading")
                self.model = AutoModelForCausalLM.from_pretrained(
                    model_id,
                    torch_dtype=torch.float16,
                    device_map="auto",
                    token=self.token
                )
            else:
                logger.info("Using CPU for model loading")
                self.model = AutoModelForCausalLM.from_pretrained(
                    model_id,
                    torch_dtype=torch.float32,
                    low_cpu_mem_usage=True,
                    token=self.token
                )
                self.model.to(self.device)

            self.tokenizer = AutoTokenizer.from_pretrained(
                model_id,
                token=self.token
            )
            logger.info("LlamaGuard model loaded successfully")

        except Exception as e:
            logger.error(f"Error loading LlamaGuard model: {str(e)}")
            raise

    def clean_analysis_output(self, text: str) -> str:
        text = text.replace("<|eot_id|>", "").replace("<|endoftext|>", "")
        text = text.replace("\n", " ")
        text = " ".join(text.split())
        text = text.strip()
        if text.startswith("unsafe"):
            text = text.replace("unsafe ", "unsafe,")
        return text.strip()

    def analyze_content(
            self,
            text: str,
            categories: Optional[List[str]] = None,
            excluded_categories: Optional[List[str]] = None
    ) -> str:
        try:
            logger.info(f"Analyzing text: '{text[:100]}{'...' if len(text) > 100 else ''}'")

            conversation = [
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "text",
                            "text": text
                        }
                    ]
                }
            ]

            kwargs = {"return_tensors": "pt"}
            if categories:
                cats_dict = {cat: cat for cat in categories}
                kwargs["categories"] = cats_dict
            if excluded_categories:
                kwargs["excluded_category_keys"] = excluded_categories

            input_ids = self.tokenizer.apply_chat_template(
                conversation,
                **kwargs
            ).to(self.device)

            with torch.inference_mode():
                prompt_len = input_ids.shape[-1]
                output = self.model.generate(
                    input_ids,
                    max_new_tokens=256,
                    pad_token_id=0,
                )

            analysis = self.tokenizer.decode(
                output[0][prompt_len:],
                skip_special_tokens=True,
                clean_up_tokenization_spaces=True
            )
            clean_analysis = self.clean_analysis_output(analysis)
            logger.info(f"Analysis completed. Result: {clean_analysis}")
            return clean_analysis

        except Exception as e:
            logger.error(f"Error during analysis: {e}")
            raise


analyzer = None
try:
    logger.info("Initializing LlamaGuard analyzer...")
    analyzer = LlamaGuardAnalyzer()
    logger.info("LlamaGuard analyzer initialized successfully")
except Exception as e:
    logger.error(f"Failed to initialize LlamaGuard analyzer: {str(e)}")


def handler(text: str, threshold: float, config: Dict[str, Any]) -> Dict[str, Any]:
    try:
        if analyzer is None:
            raise RuntimeError("LlamaGuard analyzer not initialized")

        categories = config.get('categories', [])
        excluded_categories = config.get('excluded_categories', [])

        analysis = analyzer.analyze_content(
            text,
            categories=categories,
            excluded_categories=excluded_categories
        )

        is_unsafe = not analysis.lower().startswith('safe')
        score = 1.0 if is_unsafe else 0.0

        violated_categories = []
        if is_unsafe:
            for category in ["S1", "S2", "S3", "S4", "S5", "S6", "S7",
                             "S8", "S9", "S10", "S11", "S12", "S13"]:
                if category in analysis:
                    violated_categories.append(category)

        return {
            "check_result": score > threshold,
            "score": score,
            "details": {
                "is_safe": not is_unsafe,
                "violated_categories": violated_categories,
                "raw_analysis": analysis
            }
        }

    except Exception as e:
        logger.error(f"Error in LlamaGuard analysis: {str(e)}")
        return {
            "check_result": False,
            "score": 0.0,
            "details": {"error": str(e)}
        }
