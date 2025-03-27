"""
Personally Identifiable Information (PII) Detection and Anonymization Module

This module provides functionality for detecting and anonymizing PII in text using the Presidio library.
It supports both rule-based and LLM-based detection methods.
Key Components:
    - PIIConfig: Pydantic model for configuration validation
    - PIIResult: Pydantic model for standardized result output
    - PIIService: Main service class handling detection and anonymization
    - handler: FastAPI compatible entry point
Dependencies:
    - presidio_analyzer: For PII detection
    - presidio_anonymizer: For PII anonymization
    - pydantic: For data validation
    - logging: For structured logging
"""
import logging
from typing import List, Tuple, Optional
from pydantic import BaseModel, Field
from presidio_analyzer import AnalyzerEngine, RecognizerRegistry
from presidio_anonymizer import AnonymizerEngine
from presidio_analyzer.nlp_engine import NlpEngineProvider
from presidio_analyzer.nlp_engine.transformers_nlp_engine import TransformersNlpEngine
import os


from utils.logger_config import setup_logger
logger = setup_logger(__name__, os.getenv('LOG_REMOTE', False))


class PIIEntity(BaseModel):
    """Model representing a detected PII entity."""
    entity_type: str
    value: str
    start: int
    end: int


class PIIConfig(BaseModel):
    """Configuration model for PII detection."""
    pii_method: str = Field(default="RuleBased", description="Detection method: 'RuleBased' or 'LLM'")
    entities: List[str] = Field(
        default=[
            "PERSON", "EMAIL_ADDRESS", "PHONE_NUMBER",
            "CREDIT_CARD", "US_SSN", "GENERIC_PII"
        ],
        description="List of PII entity types to detect"
    )
    language: str = Field(default="en", description="Language for PII detection")
    nlp_engine_name: Optional[str] = Field(default="spacy", description="NLP engine to use")
    debug: Optional[bool] = Field(default=False, description="Enable debug mode")
    engine_model_names: Optional[dict] = Field(default=None, description="Model names for different engines")
    ner_model_config: Optional[dict] = Field(default=None, description="NER model configuration")
    port: Optional[int] = Field(default=8080, description="Port for the service")


class PIIResult(BaseModel):
    """Standardized result model for PII detection."""
    match: bool
    score: float
    anonymized_content: Optional[str]
    pii_found: List[Tuple[str, str]]


class PIIService:
    """Service class for PII detection and anonymization."""

    def __init__(self, config: PIIConfig):
        print(f"Initializing PII service with config: {config.model_dump_json()}")
        self.config = config
        self.analyzer, self.anonymizer = self._initialize_engines()

    def _initialize_engines(self):
        print("Starting engine initialization")

        if self.config.nlp_engine_name == "transformers":
            print("Initializing transformer-based NLP engine")
            if not self.config.engine_model_names or 'transformers' not in self.config.engine_model_names:
                raise ValueError("Model name must be specified for transformer-based NLP engine")

            nlp_engine = TransformersNlpEngine(
                models=[{
                    "model_name": {
                        "spacy": "en_core_web_sm",
                        "transformers": self.config.engine_model_names['transformers']
                    },
                    "lang_code": self.config.language
                }]
            )
        else:
            print(f"Initializing {self.config.nlp_engine_name} NLP engine")
            if not self.config.engine_model_names or 'spacy' not in self.config.engine_model_names:
                raise ValueError("Model name must be specified for spacy engine")

            provider = NlpEngineProvider(nlp_configuration={
                "nlp_engine_name": self.config.nlp_engine_name,
                "models": [{
                    "model_name": self.config.engine_model_names[self.config.nlp_engine_name],
                    "lang_code": self.config.language
                }]
            })
            nlp_engine = provider.create_engine()

        nlp_engine.load()
        print(f"{self.config.nlp_engine_name} NLP engine loaded")

        if self.config.pii_method == "LLM":
            print("Initializing LLM-based PII detection")
            registry = RecognizerRegistry()
            registry.load_predefined_recognizers(nlp_engine=nlp_engine)
            analyzer = AnalyzerEngine(
                nlp_engine=nlp_engine,
                registry=registry,
                supported_languages=[self.config.language]
            )
            print("LLM-based analyzer engine initialized")
        else:
            print("Initializing rule-based PII detection")
            analyzer = AnalyzerEngine(
                nlp_engine=nlp_engine,
                supported_languages=[self.config.language]
            )
            print("Rule-based analyzer engine initialized")

        print(f"Loaded configurations: {self.config}")
        print("Anonymizer engine initialized")
        return analyzer, AnonymizerEngine()

    def analyze_text(self, text: str, anonymize: bool = True) -> PIIResult:
        print(f"Analyzing text (length: {len(text)})")
        results = self.analyzer.analyze(
            text=text,
            language=self.config.language,
            entities=self.config.entities if self.config.pii_method != "LLM" else None
        )
        print(f"Found {len(results)} PII entities: {results}")

        if anonymize:
            anonymized_result = self.anonymizer.anonymize(text=text, analyzer_results=results)
            anonymized_text = anonymized_result.text
            print(f"Anonymized text: {anonymized_text}")
        else:
            anonymized_text = None
            print("Skipping anonymization because action type is 'block'")

        identified_pii = [
            (result.entity_type, text[result.start:result.end])
            for result in results
        ]
        print(f"Identified PII entities: {identified_pii}")

        pii_score = len(identified_pii) / len(text.split()) if text.strip() else 0
        print(f"PII density score: {pii_score}")

        logger.info(f"PII analysis complete - Score: {pii_score}, Entities found: {len(identified_pii)}")

        # If anything found, match = True
        return PIIResult(
            match=(pii_score > 0),
            score=pii_score,
            anonymized_content=anonymized_text,
            pii_found=identified_pii
        )


def handler(text: str, threshold: float, config: dict) -> dict:
    print(f"Received raw config in handler: {config}")
    if "PIIService" in config:
        config["piiservice"] = config.pop("PIIService")

    pii_service_config = config.get('piiservice', {})
    pii_config = PIIConfig(
        pii_method=pii_service_config.get('PIIMethod', 'RuleBased'),
        entities=pii_service_config.get('ruleBased', {}).get('PIIEntities', PIIConfig().entities),
        language=pii_service_config.get('Models', {}).get('LangCode', 'en'),
        nlp_engine_name=pii_service_config.get('NLPEngineName', 'spacy'),
        debug=pii_service_config.get('debug', False),
        engine_model_names=pii_service_config.get('Models', {}).get('ModelName', {}),
        ner_model_config=pii_service_config.get('NERModelConfig', {}),
        port=pii_service_config.get('port', 8080)
    )
    print(f"Parsed PII configuration: {pii_config}")

    service = PIIService(pii_config)
    action_type = config.get("action_type", "anonimization").lower()
    if action_type == "block":
        result = service.analyze_text(text, anonymize=False)
    else:
        result = service.analyze_text(text, anonymize=True)

    logger.info(f"PII detection complete - Threshold: {threshold}, Score: {result.score}")
    return result.model_dump()
