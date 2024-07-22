import yaml
from presidio_analyzer import AnalyzerEngine, RecognizerRegistry
from presidio_anonymizer import AnonymizerEngine
from presidio_analyzer.nlp_engine import NlpEngineProvider

# Load configuration
with open('piiService.yaml', 'r') as config_file:
    config = yaml.safe_load(config_file)


# Initialize Presidio engines
def initialize_engines():
    pii_method = config.get('pii_method', 'RuleBased')

    if pii_method == 'LLM':
        def create_nlp_engine_with_transformers():
            provider = NlpEngineProvider(conf_file='piiService.yaml')
            return provider.create_engine()

        nlp_engine = create_nlp_engine_with_transformers()
        registry = RecognizerRegistry()
        registry.load_predefined_recognizers(nlp_engine=nlp_engine)
        analyzer = AnalyzerEngine(nlp_engine=nlp_engine, registry=registry)
    else:
        analyzer = AnalyzerEngine()

    anonymizer = AnonymizerEngine()
    return analyzer, anonymizer, pii_method


analyzer, anonymizer, pii_method = initialize_engines()


def anonymize_text(text):
    # Analyze the text for PII entities using the analyzer
    if pii_method == 'LLM':
        results = analyzer.analyze(text=text, language='en')
    else:
        results = analyzer.analyze(text=text, entities=config['rule_based']['pii_entities'], language='en')

    # Anonymize the detected PII entities
    anonymized_result = anonymizer.anonymize(text=text, analyzer_results=results)
    anonymized_text = anonymized_result.text

    # Extract PII information
    identified_pii = [(result.entity_type, text[result.start:result.end]) for result in results]

    return anonymized_text, identified_pii


def handler(text: str, threshold: float) -> dict:
    """
    Processes the input text for PII.

    Args:
    text (str): The text to process for PII.
    threshold (float): Not used in this plugin, but required by the plugin interface.

    Returns:
    dict: A dictionary containing the processed result.
    """
    anonymized_text, identified_pii = anonymize_text(text)

    return {
        "check_result": len(identified_pii) > 0,  # True if any PII was found
        "anonymized_content": anonymized_text,
        "pii_found": identified_pii
    }