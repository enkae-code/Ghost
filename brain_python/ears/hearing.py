# // Author: Enkae (enkae.dev@pm.me)
class WhisperEngine:
    """Stub WhisperEngine for when model loading fails or is disabled."""

    def __init__(self, model_size="tiny", device="cpu"):
        pass

    def load(self):
        pass

    def transcribe(self, audio_path):
        return ""

    def unload(self):
        pass
