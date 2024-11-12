import os

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import torch
from transformers import AutoTokenizer, AutoModelForSequenceClassification
import logging
from typing import Dict, Optional
from huggingface_hub import login, HfApi

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = FastAPI(title="PromptGuard Service")


class AnalyzeRequest(BaseModel):
    text: str
    threshold: float = 0.5
    temperature: float = 3.0 


class AnalyzeResponse(BaseModel):
    score: float
    details: Dict[str, float]
    classification: str


class PromptGuard:
    def __init__(self):
        self.token = os.getenv("HUGGINGFACE_API_KEY") #NEED TO REQUEST ACCESS FOR THE MODEL!
        if not self.token:
            raise ValueError("Token not set")

        try:
            login(token=self.token, write_permission=True)
            api = HfApi()
        except Exception as e:
            logger.error(f"Authentication error: {str(e)}")
            raise

        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        logger.info(f"Using device: {self.device}")

        try:
            logger.info("Loading model")
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
            logger.info("Model loaded successfully")
        except Exception as e:
            logger.error(f"Error loading model: {e}")
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

    def get_indirect_injection_score(self, text: str, temperature: float = 3.0) -> float:

        probabilities = self.get_class_probabilities(text, temperature)
        return (probabilities[1] + probabilities[2]).item()

    def analyze_text(self, text: str, temperature: float = 3.0) -> Dict[str, any]:
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


prompt_guard: Optional[PromptGuard] = None


@app.on_event("startup")
async def startup_event():
    global prompt_guard
    try:
        prompt_guard = PromptGuard()
    except Exception as e:
        logger.error(f"Failed to initialize PromptGuard: {e}")
        raise


@app.post("/analyze", response_model=AnalyzeResponse)
async def analyze_prompt(request: AnalyzeRequest):
    try:
        if not prompt_guard:
            raise HTTPException(status_code=500, detail="PromptGuard not initialized")

        results = prompt_guard.analyze_text(request.text, request.temperature)

        return AnalyzeResponse(
            score=results["score"],
            details=results["details"],
            classification=results["classification"]
        )

    except Exception as e:
        logger.error(f"Error processing request: {e}")
        raise HTTPException(
            status_code=500,
            detail=f"Error analyzing prompt: {str(e)}"
        )


@app.get("/health")
async def health_check():
    if not prompt_guard:
        raise HTTPException(status_code=503, detail="Service not ready")
    return {"status": "healthy"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)