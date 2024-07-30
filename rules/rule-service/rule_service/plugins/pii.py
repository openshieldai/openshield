import logging




from presidio_analyzer import AnalyzerEngine, RecognizerRegistry

from presidio_anonymizer import AnonymizerEngine

from presidio_analyzer.nlp_engine import NlpEngineProvider



logging.basicConfig(level=logging.DEBUG)



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

    logging.debug(f"Anonymizing text: {text}")

    logging.debug(f"PII method: {pii_method}")

    logging.debug(f"Config: {config}")



    if pii_method == 'LLM':

        results = analyzer.analyze(text=text, language='en')

    else:

        entities = config.get('RuleBased', {}).get('PIIEntities', ["PERSON", "EMAIL_ADDRESS", "PHONE_NUMBER", "CREDIT_CARD", "US_SSN", "GENERIC_PII"])

        logging.debug(f"Using entities: {entities}")

        results = analyzer.analyze(text=text, entities=entities, language='en')



    logging.debug(f"Analysis results: {results}")



    anonymized_result = anonymizer.anonymize(text=text, analyzer_results=results)

    anonymized_text = anonymized_result.text



    identified_pii = [(result.entity_type, text[result.start:result.end]) for result in results]

    logging.debug(f"Identified PII: {identified_pii}")

    logging.debug(f"Anonymized text: {anonymized_text}")



    return anonymized_text, identified_pii



def handler(text: str, threshold: float, config: dict) -> dict:
    pii_service_config = config.get('piiservice', {})
    analyzer, anonymizer, pii_method = initialize_engines(pii_service_config)
    anonymized_text, identified_pii = anonymize_text(text, analyzer, anonymizer, pii_method, pii_service_config)

    pii_score = len(identified_pii) / len(text.split())  # Simple score based on PII density

    return {
        "check_result": pii_score > threshold,
        "score": pii_score,
        "anonymized_content": anonymized_text,
        "pii_found": identified_pii
    }
