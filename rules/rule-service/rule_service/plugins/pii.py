import yaml
from presidio_analyzer import AnalyzerEngine, RecognizerRegistry
from presidio_anonymizer import AnonymizerEngine
from presidio_analyzer.nlp_engine import NlpEngineProvider

def initialize_engines(config):
    pii_method = config.get('pii_method', 'RuleBased')

    if pii_method == 'LLM':
        def create_nlp_engine_with_transformers():
            provider = NlpEngineProvider(conf=config)
            return provider.create_engine()

        nlp_engine = create_nlp_engine_with_transformers()
        registry = RecognizerRegistry()
        registry.load_predefined_recognizers(nlp_engine=nlp_engine)
        analyzer = AnalyzerEngine(nlp_engine=nlp_engine, registry=registry)
    else:
        analyzer = AnalyzerEngine()

    anonymizer = AnonymizerEngine()
    return analyzer, anonymizer, pii_method

def anonymize_text(text, analyzer, anonymizer, pii_method, config):
    if pii_method == 'LLM':
        results = analyzer.analyze(text=text, language='en')
    else:
        results = analyzer.analyze(text=text, entities=config['rule_based']['pii_entities'], language='en')

    anonymized_result = anonymizer.anonymize(text=text, analyzer_results=results)
    anonymized_text = anonymized_result.text

    identified_pii = [(result.entity_type, text[result.start:result.end]) for result in results]

    return anonymized_text, identified_pii

def handler(text: str, threshold: float, config: dict) -> dict:
    analyzer, anonymizer, pii_method = initialize_engines(config)
    anonymized_text, identified_pii = anonymize_text(text, analyzer, anonymizer, pii_method, config)

    return {
        "check_result": len(identified_pii) > 0,
        "anonymized_content": anonymized_text,
        "pii_found": identified_pii
    }