# // Author: Enkae (enkae.dev@pm.me)
class WhisperEngine:
    """Stub WhisperEngine for when voice is unavailable."""

    def __init__(self, model_size="tiny.en", device="cpu"):
        self.model_size = model_size
        self.device = device

    def load(self):
        raise RuntimeError("Voice disabled — type commands instead")

    def transcribe(self, audio_path):
        return ""

    def unload(self):
        pass
