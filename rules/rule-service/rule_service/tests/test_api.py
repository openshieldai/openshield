import unittest
from dotenv import load_dotenv
from pathlib import Path
import requests
import threading
import time
import uvicorn
import sys
import os
import logging

# Set up logging
logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger(__name__)

# Add the parent directory to the Python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from main import app

API_URL = "http://127.0.0.1:8000/rule/execute"
current_dir = Path(__file__).resolve().parent
dotenv_path = current_dir.parent.parent.parent.parent / '.env'
load_dotenv(dotenv_path)
API_KEY = os.getenv("HUGGINGFACE_API_KEY")


def run_server():
    uvicorn.run(app, host="127.0.0.1", port=8000)


class TestAPIEndpoint(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Start the server in a separate thread
        cls.server_thread = threading.Thread(target=run_server, daemon=True)
        cls.server_thread.start()
        # Wait for the server to start
        time.sleep(1)

    @classmethod
    def tearDownClass(cls):
        # Shutdown logic if needed
        pass

    @unittest.skipIf(not API_KEY, "HuggingFace API key not set")
    def test_detect_english(self):
        # Test case 1: English text
        payload = {
            "prompt": {
                "model": "",
                "messages": [{"role": "user", "content": "This is an English sentence."}]
            },
            "config": {
                "PluginName": "detect_english",
                "Threshold": 0.5,
                "api_key": API_KEY,
                "url": "https://api-inference.huggingface.co/models/papluca/xlm-roberta-base-language-detection"
            }
        }

        logger.debug(f"Sending payload: {payload}")

        response = requests.post(API_URL, json=payload)

        logger.debug(f"Response status code: {response.status_code}")
        logger.debug(f"Response content: {response.text}")

        self.assertEqual(response.status_code, 200)
        result = response.json()
        self.assertTrue(result['match'])
        self.assertGreater(result['inspection']['score'], 0.5)

        # Test case 2: Non-English text
        payload['prompt']['messages'][0]['content'] = "Dies ist ein deutscher Satz."

        logger.debug(f"Sending payload: {payload}")

        response = requests.post(API_URL, json=payload)

        logger.debug(f"Response status code: {response.status_code}")
        logger.debug(f"Response content: {response.text}")

        self.assertEqual(response.status_code, 200)
        result = response.json()
        self.assertFalse(result['match'])
        self.assertLess(result['inspection']['score'], 0.5)


if __name__ == '__main__':
    unittest.main()
