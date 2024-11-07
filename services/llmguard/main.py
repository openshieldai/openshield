# llamaguard_service.py
import os
import logging
from typing import Dict, List, Optional
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
import torch
from transformers import AutoModelForCausalLM, AutoTokenizer
from huggingface_hub import login, HfApi
import uvicorn
from dotenv import load_dotenv

load_dotenv()


logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)


class AnalyzeRequest(BaseModel):
    text: str
    categories: List[str] = Field(default_factory=list)
    excluded_categories: List[str] = Field(default_factory=list)


class AnalyzeResponse(BaseModel):
    response: str


class LlamaGuard:
    def __init__(self):
        self.token = os.getenv("HUGGIGNFACE_API_KEY")
        if not self.token:
            raise ValueError("HuggingFace API token not set")

        try:
            login(token=self.token, write_permission=True)
            self.api = HfApi()
        except Exception as e:
            logger.error(f"Authentication error: {str(e)}")
            raise

        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        logger.info(f"Using device: {self.device}")

        try:
            logger.info("Loading Llama Guard model...")
            model_id = "meta-llama/Llama-Guard-3-1B"

            self.model = AutoModelForCausalLM.from_pretrained(
                model_id,
                torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
                device_map="auto",
                token=self.token
            )
            self.tokenizer = AutoTokenizer.from_pretrained(
                model_id,
                token=self.token
            )
            logger.info("Model loaded successfully")

        except Exception as e:
            logger.error(f"Error loading model: {e}")
            raise

    def clean_analysis_output(self, text: str) -> str:

        text = text.replace("<|eot_id|>", "").replace("<|endoftext|>", "")
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

            analysis = self.tokenizer.decode(output[0][prompt_len:], skip_special_tokens=True)
            clean_analysis = self.clean_analysis_output(analysis)

            logger.info(f"Analysis completed. Result: {clean_analysis}")
            return clean_analysis

        except Exception as e:
            logger.error(f"Error during analysis: {e}")
            raise


app = FastAPI(
    title="LlamaGuard ",
    description="Meta's Llama Guard model"
)

llama_guard: Optional[LlamaGuard] = None


@app.on_event("startup")
async def startup_event():
    global llama_guard
    try:
        llama_guard = LlamaGuard()
    except Exception as e:
        logger.error(f"Failed to initialize LlamaGuard: {e}")
        raise


@app.post("/analyze", response_model=AnalyzeResponse)
async def analyze_content(request: AnalyzeRequest):
    try:
        if not llama_guard:
            raise HTTPException(status_code=500, detail="LlamaGuard not initialized")

        response = llama_guard.analyze_content(
            request.text,
            request.categories,
            request.excluded_categories
        )

        return AnalyzeResponse(response=response)

    except Exception as e:
        logger.error(f"Error processing request: {e}")
        raise HTTPException(
            status_code=500,
            detail=f"Error analyzing content: {str(e)}"
        )


@app.get("/health")
async def health_check():
    if not llama_guard:
        raise HTTPException(status_code=503, detail="Service not ready")
    return {"status": "healthy"}


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)