from __future__ import annotations

import os
from dataclasses import dataclass


def _env_float(name: str, default: float) -> float:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _env_int(name: str, default: int) -> int:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


@dataclass(slots=True)
class Settings:
    groq_api_key: str | None = None
    groq_model: str = "groq/compound"
    groq_model_version: str = "latest"
    request_timeout_seconds: float = 30.0
    groq_timeout_seconds: float = 20.0
    max_request_bytes: int = 1_048_576

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            groq_api_key=os.getenv("GROQ_API_KEY", "").strip() or None,
            groq_model=os.getenv("GROQ_MODEL", "groq/compound").strip() or "groq/compound",
            groq_model_version=os.getenv("GROQ_MODEL_VERSION", "latest").strip() or "latest",
            request_timeout_seconds=_env_float("AI_REQUEST_TIMEOUT_SECONDS", 30.0),
            groq_timeout_seconds=_env_float("GROQ_TIMEOUT_SECONDS", 20.0),
            max_request_bytes=_env_int("AI_MAX_REQUEST_BYTES", 1_048_576),
        )

