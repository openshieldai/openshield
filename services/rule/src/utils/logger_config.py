import logging
import os
import json

def setup_logger(name, is_remote=False):
    logger = logging.getLogger(name)

    # Only configure if handlers haven't been set up
    if not logger.handlers:
        log_level = os.getenv('LOG_LEVEL', 'INFO').upper()
        numeric_level = getattr(logging, log_level, None)

        if not isinstance(numeric_level, int):
            raise ValueError(f'Invalid log level: {log_level}')

        json_formatter = logging.Formatter(
            '{"timestamp":"%(asctime)s", "level":"%(levelname)s", "message":"%(message)s", '
            '"name":"%(name)s", "filename":"%(filename)s", "lineno":%(lineno)d}'
        )

        if is_remote and os.getenv('LOG_REMOTE_HOST'):
            handler = logging.handlers.SysLogHandler(address = (os.getenv('LOG_REMOTE_HOST', "127.0.0.1"), os.getenv('LOG_REMOTE_PORT', 514)))
        else:
            handler = logging.StreamHandler()

        handler.setFormatter(json_formatter)
        handler.setFormatter(json_formatter)
        logger.setLevel(numeric_level)
        logger.addHandler(handler)

    return logger
