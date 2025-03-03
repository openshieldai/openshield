"""
This module provides a content safety checking system that analyzes text for inappropriate or harmful content.

The module uses a multi-tiered classification system to detect and categorize potentially problematic content
across different severity levels (mild, moderate, severe) and categories (profanity, hate speech, etc.).

The `ContentCategories` class defines comprehensive word lists for different types of inappropriate content,
organized by severity level and category.

Classes:
- ContentCategories: Contains categorized word lists organized by content type and severity level.
    Categories include:
    - PROFANITY_VULGAR: Common swear words and vulgar language
    - HATE_SPEECH: Discriminatory and hateful content
    - SEXUAL_EXPLICIT: Adult and sexually explicit content
    - VIOLENCE_HARMFUL: Violent and self-harm related content
    - POLITICAL: Politically sensitive or extreme content

Functions:
- calculate_severity_score: Calculates a severity score based on matched content and its category.
- check_content: Analyzes text against all content categories and returns detailed results.
- handler: Main entry point that processes text and returns content safety analysis results.

Returns:
    dict: {
        "check_result": bool,  # Whether content exceeds safety threshold
        "score": float,        # Severity score (0.0 to 1.0)
        "details": dict        # Detailed breakdown of matches by category and severity
    }

Severity Scoring:
- Mild violations: 0.3
- Moderate violations: 0.6
- Severe violations: 1.0

Dependencies:
- re: Python's regular expression module
- logging: Python's logging facility
"""

import re
import logging

from utils.logger_config import setup_logger
logger = setup_logger(__name__)

class ContentCategories:
    PROFANITY_VULGAR = {
        'mild': [
            "arse", "ass", "damn", "dick", "piss", "pissed", "crap", "hell", "bugger", "bloody", "bollocks", "wuss"
        ],
        'moderate': [
            "bastard", "bitch", "fuck", "fucking", "fuckin", "motherfucker", "shit",
            "asshole", "pussy", "penis", "vagina", "cock", "boobs", "tits", "titties",
            "douchebag", "prick", "whore", "slut", "jackass", "balls", "nuts", "schlong"
        ],
        'severe': [
            "cunt", "dildo", "blowjob", "cum", "ejaculate", "jerk-off", "masturbate",
            "wanker", "shag", "twat", "ballsack", "boner", "muff", "nutsack",
            "sucker", "lick", "licker", "fuckface", "dumbass", "fuckhead",
            "shithead", "shitface", "cumslut", "cumbucket", "semen", "pecker"
        ]
    }

    HATE_SPEECH = {
        'mild': [
            "homo", "jew", "jewish", "muslim", "muslims", "queer", "black", "whitey", "cracker"
        ],
        'moderate': [
            "homophobic", "racist", "anti-semitic", "islamophobe", "homophobe",
            "bigot", "xenophobe", "hate speech", "antisemitism"
        ],
        'severe': [
            "chink", "nigga", "nigger", "coon", "negro", "faggot", "dyke",
            "nazi", "jap", "sandbar", "mongoloid", "furfag", "coont",
            "wetback", "spic", "gook", "kike", "towelhead", "raghead"
        ]
    }

    SEXUAL_EXPLICIT = {
        'mild': [
            "porn", "smut", "erotic", "nudes", "naked", "sexy", "kinky", "innuendo"
        ],
        'moderate': [
            "anal", "clitoris", "clit", "pornography", "orgasm",
            "redtube", "xxx", "hardcore", "fetish", "stripper", "bondage", "lingerie", "dildo"
        ],
        'severe': [
            "buttrape", "anilingus", "cumshot", "rape", "molest",
            "cumdumpster", "cumguzzler", "gangbang", "necrophilia",
            "pedo", "pedophile", "pedophilia", "child predator",
            "loli", "lolicon", "cub", "bestiality", "incest", "rape fantasy"
        ]
    }

    VIOLENCE_HARMFUL = {
        'mild': [
            "die", "shoot", "kill", "stab", "punch", "beat", "slap", "attack", "hurt"
        ],
        'moderate': [
            "bomb", "bombing", "bombed", "shooting",
            "cliff", "bridge", "assault", "murder", "strangle", "torture", "execute", "blow up"
        ],
        'severe': [
            "terrorist", "terrorism",
            "kys", "i want to die", "cut myself", "fuck life",
            "suicide", "hang myself", "self-harm", "slit my wrists", "end it all"
        ]
    }

    POLITICAL = {
        'mild': [
            "conservative", "liberal", "democrat", "republican", "leftist", "right-wing"
        ],
        'moderate': [
            "trump", "maga", "make america great again", "biden", "antifa", "patriot", "socialism", "capitalism", "communist", "fascist"
        ],
        'severe': [
            "far right", "isis", "white supremacist", "neo-nazi", "kkk", "alt-right", "anarchist", "extremist"
        ]
    }


def calculate_severity_score(matches: set, category_levels: dict) -> tuple[float, dict]:
    """Calculate severity score and categorize matched words."""
    severity_weights = {
        'mild': 0.3,
        'moderate': 0.6,
        'severe': 1.0
    }

    results = {
        'mild': [],
        'moderate': [],
        'severe': []
    }

    max_score = 0

    for severity, words in category_levels.items():
        matched = [word for word in words if word in matches]
        if matched:
            results[severity] = matched
            max_score = max(max_score, severity_weights[severity])

    return max_score, results

def check_content(text: str) -> tuple[float, dict]:
    """Check text content against all categories."""
    text = text.lower()
    results = {}
    max_total_score = 0

    for category_name, category_levels in ContentCategories.__dict__.items():
        if category_name.startswith('_'):
            continue

        # Create pattern for all words in this category
        all_words = [word for level in category_levels.values() for word in level]
        pattern = re.compile(r'\b(' + '|'.join(map(re.escape, all_words)) + r')\b', re.IGNORECASE)

        matches = set(pattern.findall(text))
        if matches:
            score, severity_matches = calculate_severity_score(matches, category_levels)
            max_total_score = max(max_total_score, score)

            results[category_name] = {
                'score': score,
                'matches': severity_matches
            }

    return max_total_score, results

def handler(text: str, threshold: float, _: dict) -> dict:
    """Handle content safety check request."""
    score, details = check_content(text)

    if details:
        logging.warning(f"Content safety issues found (score: {score:.2f}):")
        for category, result in details.items():
            logging.warning(f"- {category}: {result}")
    else:
        logging.debug("No content safety issues found")

    return {
        "check_result": score >= threshold,
        "score": score,
        "details": details
    }
