class AudioRecorder:
    """Stub AudioRecorder for when voice hardware is unavailable."""

    def start_recording(self):
        return False

    def record_chunk(self):
        pass

    def stop_recording(self):
        return None

    def cleanup(self):
        pass
