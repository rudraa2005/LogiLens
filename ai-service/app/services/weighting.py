from __future__ import annotations

from dataclasses import dataclass

from app.schemas import MLPredictions, WeightProfile


def clamp(value: float, minimum: float = 0.0, maximum: float = 100.0) -> float:
    if value < minimum:
        return minimum
    if value > maximum:
        return maximum
    return value


def normalize_score(value: float | None) -> float:
    if value is None:
        return 0.0
    if value <= 0:
        return 0.0
    if value <= 1.0:
        return value
    if value <= 100.0:
        return value / 100.0
    return 1.0


def severity(factor: float) -> float:
    if factor <= 1.0:
        return 0.0
    return min(1.5, factor - 1.0)


def normalize_weights(weights: dict[str, float]) -> WeightProfile:
    traffic = max(0.0, weights.get("traffic", 0.0))
    news = max(0.0, weights.get("news", 0.0))
    weather = max(0.0, weights.get("weather", 0.0))
    total = traffic + news + weather
    if total <= 0:
        return WeightProfile(traffic=1 / 3, news=1 / 3, weather=1 / 3)
    return WeightProfile(
        traffic=traffic / total,
        news=news / total,
        weather=weather / total,
    )


@dataclass(slots=True)
class WeightAdjustment:
    profile: WeightProfile
    summary: str


class WeightAdjuster:
    def adjust(
        self,
        base_weights: dict[str, float],
        ml_predictions: MLPredictions | None,
        insight_hints: dict[str, float] | None,
        feedback_bias: float = 0.0,
    ) -> WeightAdjustment:
        weights = dict(base_weights)
        ml_predictions = ml_predictions or MLPredictions()
        insight_hints = insight_hints or {}

        traffic_multiplier = 1.0 + normalize_score(
            ml_predictions.traffic_trend
            or ml_predictions.delay_probability
            or insight_hints.get("traffic")
        ) * 0.75
        news_multiplier = 1.0 + normalize_score(
            ml_predictions.news_risk or insight_hints.get("news")
        ) * 0.80
        weather_multiplier = 1.0 + normalize_score(
            ml_predictions.weather_risk or insight_hints.get("weather")
        ) * 0.65

        if ml_predictions.route_reliability is not None:
            reliability = normalize_score(ml_predictions.route_reliability)
            traffic_multiplier *= max(0.75, 1.0 - reliability * 0.15)
            news_multiplier *= max(0.75, 1.0 - reliability * 0.10)
            weather_multiplier *= max(0.75, 1.0 - reliability * 0.10)

        weights["traffic"] = weights.get("traffic", 0.0) * traffic_multiplier
        weights["news"] = weights.get("news", 0.0) * news_multiplier * max(0.8, 1.0+feedback_bias)
        weights["weather"] = weights.get("weather", 0.0) * weather_multiplier
        weights["traffic"] = weights.get("traffic", 0.0) * max(0.8, 1.0+feedback_bias*0.5)

        profile = normalize_weights(weights)
        summary = (
            f"ML adjusted weights to traffic {profile.traffic:.2f}, "
            f"news {profile.news:.2f}, weather {profile.weather:.2f}."
        )
        return WeightAdjustment(profile=profile, summary=summary)
