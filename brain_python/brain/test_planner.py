import unittest
from unittest.mock import MagicMock, patch
import sys
import json

# Mock dependencies before importing planner
sys.modules['ollama'] = MagicMock()
sys.modules['sentence_transformers'] = MagicMock()

from brain_python.brain.planner import GhostPlanner

class TestGhostPlanner(unittest.TestCase):
    def setUp(self):
        self.planner = GhostPlanner()
        # Mock the load_kernel_token method to avoid file read errors
        self.planner._load_kernel_token = MagicMock(return_value="test_token")
        # Disable RAG to avoid socket calls
        self.planner._rag_enabled = False

    @patch('brain_python.brain.planner.ollama')
    def test_decide_ollama_available(self, mock_ollama):
        # Setup mock response
        mock_response = MagicMock()
        mock_response.message.content = '{"intent": "test", "plan": ["step1"], "actions": [{"type": "WAIT", "duration": 1}]}'
        mock_ollama.chat.return_value = mock_response

        # Call decide
        result = self.planner.decide("test input")

        # Verify result
        self.assertIsInstance(result, dict)
        self.assertEqual(result['intent'], 'test')
        self.assertEqual(len(result['actions']), 1)
        self.assertEqual(result['actions'][0]['type'], 'WAIT')

    @patch('brain_python.brain.planner.ollama')
    def test_decide_ollama_unavailable(self, mock_ollama):
        # Simulate Ollama error
        mock_ollama.chat.side_effect = Exception("Connection refused")

        # Call decide
        result = self.planner.decide("test input")

        # Verify result - should handle error gracefully
        self.assertIsInstance(result, dict)
        self.assertIn('error', result)
        # It should return the "No response from LLM" error because the exception is caught
        # and result remains None
        self.assertEqual(result['error'], "No response from LLM (Ollama offline?)")

    @patch('brain_python.brain.planner.ollama')
    def test_decide_ollama_none(self, mock_ollama):
        # Simulate Ollama module not imported
        with patch('brain_python.brain.planner.ollama', None):
            result = self.planner.decide("test input")
            self.assertIsInstance(result, dict)
            self.assertEqual(result['error'], "No response from LLM (Ollama offline?)")

if __name__ == '__main__':
    unittest.main()
