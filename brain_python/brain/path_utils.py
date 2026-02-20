# // Author: Enkae (enkae.dev@pm.me)
"""Shared path validation utilities for Ghost."""
from pathlib import Path


def is_safe_path(path_str: str) -> bool:
    """
    Validates that a path is relative and stays within safe boundaries.
    Prevents path traversal attacks (e.g., ../../windows/system32).
    """
    try:
        path = Path(path_str)
        # Prevent absolute paths (e.g. C:\ or /etc)
        if path.is_absolute():
            return False

        # Resolve path relative to current working directory
        # We use current directory as the root sandbox
        base_dir = Path.cwd().resolve()
        target_path = (base_dir / path).resolve()

        # Check if target path is within base_dir
        # Note: is_relative_to is available in Python 3.9+
        return target_path.is_relative_to(base_dir)
    except Exception:
        return False
