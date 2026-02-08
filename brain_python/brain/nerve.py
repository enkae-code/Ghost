# // Author: Enkae (enkae.dev@pm.me)
"""
Nerve - The gRPC Client wrapper for the Nervous System (Kernel).
Handles connection management and type conversion.
"""

import grpc
import logging
from . import ghost_pb2
from . import ghost_pb2_grpc


class Nerve:
    """
    The gRPC Client wrapper for the Nervous System (Kernel).
    Handles connection management and type conversion.
    """
    def __init__(self, host: str = "localhost", port: int = 50051):
        self.target = f"{host}:{port}"
        self.channel = None
        self.stub = None
        self.logger = logging.getLogger("Nerve")

    def connect(self) -> None:
        """Establish the gRPC channel."""
        # For localhost, insecure is standard. 
        # In Phase 2 (Enclave), we would switch to secure_channel with SSL.
        self.channel = grpc.insecure_channel(self.target)
        self.stub = ghost_pb2_grpc.NervousSystemStub(self.channel)
        self.logger.info(f"Connected to Nervous System at {self.target}")

    def request_permission(self, intent: str, actions: list, trace_id: str) -> dict:
        """
        Asks the Conscience for permission to act.
        Returns a dict compatible with the existing main.py logic.
        """
        if not self.stub:
            self.connect()

        # Convert simple list of dicts -> Proto Action objects
        proto_actions = []
        for act in actions:
            # Flatten payload: Ensure all values are strings for the map<string, string>
            payload = {k: str(v) for k, v in act.items() if k != "type"}
            proto_actions.append(ghost_pb2.Action(
                type=act.get("type", "UNKNOWN"),
                payload=payload
            ))

        req = ghost_pb2.PermissionRequest(
            intent=intent,
            actions=proto_actions,
            trace_id=trace_id
        )

        try:
            resp = self.stub.RequestPermission(req)
            return {
                "approved": resp.approved,
                "reason": resp.reason,
                "trust_score": resp.trust_score
            }
        except grpc.RpcError as e:
            self.logger.error(f"Nerve Damage (RPC Error): {e.code()} - {e.details()}")
            # Fail-safe: If Conscience is down, we must default to blocking dangerous actions
            # or return a specific error code for main.py to handle.
            return {"approved": False, "reason": f"Conscience Unreachable: {e.code()}"}

    def close(self) -> None:
        """Close the gRPC channel."""
        if self.channel:
            self.channel.close()
            self.logger.info("Nerve connection closed")
