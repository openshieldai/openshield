import unicodedata
import logging

logging.basicConfig(level=logging.DEBUG)


def contains_unicode(text: str) -> bool:
    return any(ord(char) > 127 for char in text)


class InvisibleText:
    def __init__(self):
        self.banned_categories = ["Cf", "Co", "Cn"]
        self.chars = []

    def check(self, text: str) -> bool:
        for char in text:
            if unicodedata.category(char) in self.banned_categories:
                self.chars.append(char)
        return bool(self.chars)


def handler(text: str, threshold: float, config: dict) -> dict:
    inv_text = InvisibleText()
    if not contains_unicode(text):
        return {
            "check_result": 0 > threshold,
            "score": 0,
        }

    if inv_text.check(text):
        logging.warning("Found invisible characters in the prompt: %s", inv_text.chars)
        return {
            "check_result": 1 > threshold,
            "score": 1,
        }

    logging.debug("No invisible characters found")
    return {
        "check_result": 0 > threshold,
        "score": 0,
    }
