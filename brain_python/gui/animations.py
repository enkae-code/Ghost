# // Author: Enkae (enkae.dev@pm.me)
from enum import Enum


class TrayState(Enum):
    IDLE = "idle"
    PULSE = "pulse"
    BUSY = "busy"
