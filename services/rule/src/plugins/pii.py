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

    class Config:
        # Disable protected namespace checks if you prefer to keep the original field name
        protected_namespaces = ()

class PIIResult(BaseModel):
    """Standardized result model for PII detection."""
    check_result: bool
    score: float
    anonymized_content: str
    pii_found: List[Tuple[str, str]]

class PIIService:
    """Service class for PII detection and anonymization."""

    def __init__(self, config: PIIConfig):
        """Initialize PII detection engines based on configuration."""
        print(f"Initializing PII service with config: {config.model_dump_json()}")

        self.config = config
        self.analyzer, self.anonymizer = self._initialize_engines()

    def _initialize_engines(self) -> Tuple[AnalyzerEngine, AnonymizerEngine]:
        """Initialize the analyzer and anonymizer engines."""
        print("Starting engine initialization")

        if self.config.nlp_engine_name == "transformers":
            print("Initializing transformer-based NLP engine")
            if not self.config.engine_model_names or 'transformers' not in self.config.engine_model_names:
                raise ValueError("Model name must be specified for transformer-based NLP engine")

            nlp_engine = TransformersNlpEngine(
                models=[{
                    "model_name": {
                        "spacy": "en_core_web_sm",  # Required base model
                        "transformers": self.config.engine_model_names['transformers']
                    },
                    "lang_code": self.config.language
                }]
            )
        else:
            # For other engines (e.g., spacy), use the standard NlpEngineProvider
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

        nlp_engine.load()  # Load the model
        print(f"{self.config.nlp_engine_name} NLP engine loaded")

        if self.config.pii_method == "LLM":
            print(f"Initializing LLM-based PII detection")
            registry = RecognizerRegistry()
            registry.load_predefined_recognizers(nlp_engine=nlp_engine)
            analyzer = AnalyzerEngine(
                nlp_engine=nlp_engine,
                registry=registry,
                supported_languages=[self.config.language]
            )
            print("LLM-based analyzer engine initialized")
        else:
            print(f"Initializing rule-based PII detection")
            analyzer = AnalyzerEngine(
                nlp_engine=nlp_engine,
                supported_languages=[self.config.language]
            )
            print("Rule-based analyzer engine initialized")

        print(f"Loaded configurations: {self.config}")
        print("Anonymizer engine initialized")
        return analyzer, AnonymizerEngine()

    def analyze_text(self, text: str) -> PIIResult:
        """
        Analyze text for PII content and return anonymized result.

        Args:
            text: Input text to analyze

        Returns:
            PIIResult containing detection results and anonymized text
        """
        print(f"Analyzing text (length: {len(text)})")

        # Analyze text for PII
        results = self.analyzer.analyze(
            text=text,
            language=self.config.language,
            entities=self.config.entities if self.config.pii_method != "LLM" else None
        )

        print(f"Found {len(results)} PII entities: {results}")

        # Anonymize detected PII
        anonymized_result = self.anonymizer.anonymize(text=text, analyzer_results=results)
        print(f"Anonymized text: {anonymized_result.text}")

        # Extract identified PII entities
        identified_pii = [
            (result.entity_type, text[result.start:result.end])
            for result in results
        ]
        print(f"Identified PII entities: {identified_pii}")

        # Calculate PII density score
        pii_score = len(identified_pii) / len(text.split()) if text else 0
        print(f"PII density score: {pii_score:.2f}")

        logger.info(f"PII analysis complete - Score: {pii_score:.2f}, Entities found: {len(identified_pii)}")

        return PIIResult(
            check_result=pii_score > 0,
            score=pii_score,
            anonymized_content=anonymized_result.text,
            pii_found=identified_pii
        )

def handler(text: str, threshold: float, config: dict) -> dict:
    """
    FastAPI compatible handler function for PII detection.
    """
    print(f"Received raw config in handler: {config}")
    if "PIIService" in config:
        config["piiservice"] = config.pop("PIIService")

    # Get the PIIService configuration
    pii_service_config = config.get('piiservice', {})

    # Parse configuration with proper nesting
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

    # Initialize service and analyze text
    service = PIIService(pii_config)
    result = service.analyze_text(text)

    logger.info(f"PII detection complete - Threshold: {threshold}, Score: {result.score}")

    return result.model_dump()
