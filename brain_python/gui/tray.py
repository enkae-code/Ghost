import threading


class TrayIcon:
    """Stub TrayIcon for headless/demo mode. Replaces pystray system tray."""

    def __init__(self, title="Ghost"):
        self.title = title
        self._quit_handler = None
        self._stop_event = threading.Event()

    def set_state(self, state):
        pass

    def set_quit_handler(self, handler):
        self._quit_handler = handler

    def run(self):
        # Block until stop() is called — mirrors real tray behavior
        self._stop_event.wait()

    def stop(self):
        self._stop_event.set()
