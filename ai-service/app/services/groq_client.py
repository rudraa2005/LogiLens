from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import Any

import httpx

from app.config import Settings
from app.schemas import AnalysisRequest, InsightReport

LOGGER = logging.getLogger(__name__)


def _strip_json(text: str) -> str:
    text = text.strip()
    if text.startswith("```"):
        text = text.removeprefix("```json").removeprefix("```").strip()
        if text.endswith("```"):
            text = text[:-3].strip()
    start = text.find("{")
    end = text.rfind("}")
    if start >= 0 and end > start:
        return text[start : end + 1]
    return text


def _coerce_float(value: Any, default: float = 0.0) -> float:
    try:
        if value is None:
            return default
        return float(value)
    except (TypeError, ValueError):
        return default


def _coerce_text(value: Any, default: str = "") -> str:
    if value is None:
        return default
    text = str(value).strip()
    return text or default


def _coerce_list(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    items: list[str] = []
    for item in value:
        text = _coerce_text(item)
        if text:
            items.append(text)
    return items


@dataclass(slots=True)
class GroqInsightClient:
    settings: Settings
    _client: httpx.AsyncClient

    @classmethod
    def create(cls, settings: Settings) -> "GroqInsightClient":
        return cls(settings=settings, _client=httpx.AsyncClient(timeout=settings.groq_timeout_seconds))

    async def aclose(self) -> None:
        await self._client.aclose()

    async def analyze(self, mode: str, request: AnalysisRequest, route_summary: str) -> InsightReport:
        if not self.settings.groq_api_key:
            return self._fallback_report(mode, request, route_summary, "Groq API key is not configured.")

        location = self._resolve_location(request, route_summary)
        prompt = self._build_prompt(mode, location, route_summary, request)
        payload: dict[str, Any] = {
            "model": self.settings.groq_model,
            "messages": [
                {
                    "role": "system",
                    "content": (
                        "You are a logistics intelligence analyst. "
                        "Use live web search when needed. Return strict JSON only."
                    ),
                },
                {"role": "user", "content": prompt},
            ],
        }

        if self.settings.groq_model.startswith("groq/compound"):
            payload["compound_custom"] = {"tools": {"enabled_tools": ["web_search"]}}

        headers = {
            "Authorization": f"Bearer {self.settings.groq_api_key}",
            "Content-Type": "application/json",
        }
        if self.settings.groq_model.startswith("groq/compound"):
            headers["Groq-Model-Version"] = self.settings.groq_model_version

        try:
            response = await self._client.post(
                "https://api.groq.com/openai/v1/chat/completions",
                headers=headers,
                json=payload,
            )
            response.raise_for_status()
            data = response.json()
            content = (
                data.get("choices", [{}])[0]
                .get("message", {})
                .get("content", "")
            )
            parsed = self._parse_report(content)
            if parsed is not None:
                return parsed
            return self._fallback_report(mode, request, route_summary, content or "Groq returned an empty response.")
        except Exception as exc:  # pragma: no cover - network failures are environment-specific
            LOGGER.warning("Groq insight lookup failed: %s", exc)
            return self._fallback_report(mode, request, route_summary, f"Live lookup unavailable: {exc}")

    def _build_prompt(self, mode: str, location: str, route_summary: str, request: AnalysisRequest) -> str:
        context_lines = []
        if request.context.location_name:
            context_lines.append(f"Context location: {request.context.location_name}")
        if request.alternatives:
            context_lines.append(f"Alternatives provided: {len(request.alternatives)}")
        if request.ml_predictions:
            context_lines.append(
                "ML predictions: "
                f"traffic_trend={request.ml_predictions.traffic_trend}, "
                f"news_risk={request.ml_predictions.news_risk}, "
                f"weather_risk={request.ml_predictions.weather_risk}, "
                f"route_reliability={request.ml_predictions.route_reliability}, "
                f"delay_probability={request.ml_predictions.delay_probability}, "
                f"confidence_delta={request.ml_predictions.confidence_delta}"
            )

        mode_instructions = {
            "news": "Focus on disruption reports, incidents, closures, strikes, accidents, protests, and weather events.",
            "traffic": "Focus on traffic trends, congestion, road works, closures, transit disruption, and delay patterns.",
            "confidence": "Blend news, traffic, and operational stability into a route-confidence assessment.",
        }[mode]

        return "\n".join(
            [
                f"Location: {location}",
                f"Route summary: {route_summary}",
                *context_lines,
                "",
                mode_instructions,
                "",
                "Return strict JSON with this structure:",
                "{",
                '  "risk_score": number between 0 and 100,',
                '  "confidence_score": number between 0 and 100,',
                '  "explanation": "1-3 concise sentences",',
                '  "traffic_risk": number between 0 and 100,',
                '  "news_risk": number between 0 and 100,',
                '  "weather_risk": number between 0 and 100,',
                '  "weight_hints": {"traffic": 0.5-1.5, "news": 0.5-1.5, "weather": 0.5-1.5},',
                '  "key_events": ["short phrase", "..."]',
                "}",
                "Use live search results to ground the report when available.",
            ]
        )

    def _parse_report(self, content: str) -> InsightReport | None:
        if not content:
            return None
        try:
            payload = json.loads(_strip_json(content))
        except json.JSONDecodeError:
            return None

        weight_hints = payload.get("weight_hints") if isinstance(payload, dict) else {}
        if not isinstance(weight_hints, dict):
            weight_hints = {}

        return InsightReport(
            risk_score=_coerce_float(payload.get("risk_score"), 0.0),
            confidence_score=_coerce_float(payload.get("confidence_score"), 50.0),
            explanation=_coerce_text(payload.get("explanation"), content[:400]),
            traffic_risk=_coerce_float(payload.get("traffic_risk"), _coerce_float(payload.get("risk_score"), 0.0)),
            news_risk=_coerce_float(payload.get("news_risk"), _coerce_float(payload.get("risk_score"), 0.0)),
            weather_risk=_coerce_float(payload.get("weather_risk"), _coerce_float(payload.get("risk_score"), 0.0)),
            weight_hints={
                "traffic": _coerce_float(weight_hints.get("traffic"), 1.0),
                "news": _coerce_float(weight_hints.get("news"), 1.0),
                "weather": _coerce_float(weight_hints.get("weather"), 1.0),
            },
            key_events=_coerce_list(payload.get("key_events")),
        )

    def _fallback_report(self, mode: str, request: AnalysisRequest, route_summary: str, detail: str) -> InsightReport:
        location = self._resolve_location(request, route_summary)
        local_risk = min(100.0, self._local_risk(request))
        explanation = (
            f"Live web search was unavailable for {location}. "
            f"Using local route signals for {mode}. {detail}"
        )
        return InsightReport(
            risk_score=local_risk,
            confidence_score=max(35.0, 75.0 - local_risk * 0.25),
            explanation=explanation,
            traffic_risk=local_risk,
            news_risk=local_risk,
            weather_risk=local_risk,
            weight_hints={"traffic": 1.0, "news": 1.0, "weather": 1.0},
            key_events=[f"Fallback analysis for {location}"],
        )

    def _local_risk(self, request: AnalysisRequest) -> float:
        total = 0.0
        total_weight = 0.0
        for step in request.route.steps:
            factor = request.context.edge_factors.get(step.edge_id)
            if factor is None:
                continue
            weight = self._step_weight(step.time, step.distance)
            severity = max(
                factor.traffic_factor,
                factor.weather_factor,
                factor.news_factor,
            )
            total += min(100.0, max(0.0, (severity - 1.0) * 100.0)) * weight
            total_weight += weight
        if total_weight == 0:
            return 25.0
        return total / total_weight

    def _resolve_location(self, request: AnalysisRequest, route_summary: str) -> str:
        if request.location:
            return request.location.strip()
        if request.context.location_name:
            return request.context.location_name.strip()
        return route_summary

    @staticmethod
    def _step_weight(time_value: float, distance_value: float) -> float:
        if time_value > 0:
            return time_value
        if distance_value > 0:
            return distance_value
        return 1.0

