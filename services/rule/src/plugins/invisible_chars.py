"""
This module provides a handler for detecting invisible characters in a given text.

The `InvisibleText` class checks for characters that belong to specific Unicode categories (Cf, Co, Cn) which are considered invisible or non-printable.

The `handler` function uses the `InvisibleText` class to check if the text contains any invisible characters and returns a result indicating the presence of such characters along with a score.

Classes:
- InvisibleText: Checks for invisible characters in the text based on Unicode categories.

Functions:
- contains_unicode: Checks if the text contains any Unicode characters.
- handler: Uses `InvisibleText` to check for invisible characters and returns the result.

Dependencies:
- unicodedata: Provides access to the Unicode Character Database.
- logging: Provides a way to configure and use loggers.
"""

import unicodedata
import logging
import os
from utils.logger_config import setup_logger
logger = setup_logger(__name__, os.getenv('LOG_REMOTE', False))


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


def handler(text: str, threshold: float, _: dict) -> dict:
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
