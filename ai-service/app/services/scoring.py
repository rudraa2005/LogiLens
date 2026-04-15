from __future__ import annotations

from dataclasses import dataclass

from app.schemas import AnalysisRequest, AnalysisResponse, Edge, InsightReport, Route, RouteStep
from app.services.feedback import FeedbackCalibration
from app.services.weighting import WeightAdjuster, clamp, normalize_score, severity


@dataclass(slots=True)
class EndpointProfile:
    name: str
    base_weights: dict[str, float]
    news_boost: float
    traffic_boost: float
    weather_boost: float
    confidence_bias: float


class RouteScorer:
    def __init__(self) -> None:
        self._adjuster = WeightAdjuster()

    def analyze_news(
        self,
        request: AnalysisRequest,
        insight: InsightReport,
        calibration: FeedbackCalibration | None = None,
    ) -> AnalysisResponse:
        return self._score(request, insight, self._profile_news(), calibration or FeedbackCalibration())

    def predict_traffic(
        self,
        request: AnalysisRequest,
        insight: InsightReport,
        calibration: FeedbackCalibration | None = None,
    ) -> AnalysisResponse:
        return self._score(request, insight, self._profile_traffic(), calibration or FeedbackCalibration())

    def route_confidence(
        self,
        request: AnalysisRequest,
        insight: InsightReport,
        calibration: FeedbackCalibration | None = None,
    ) -> AnalysisResponse:
        return self._score(request, insight, self._profile_confidence(), calibration or FeedbackCalibration())

    def _score(
        self,
        request: AnalysisRequest,
        insight: InsightReport,
        profile: EndpointProfile,
        calibration: FeedbackCalibration,
    ) -> AnalysisResponse:
        route = request.route
        route_distance = route.total_distance or self._compute_total_distance(route)
        route_time = route.total_time or self._compute_total_time(route)

        local_signals = self._collect_local_signals(request)
        signal_values = {
            "traffic": max(local_signals["traffic"], normalize_score(insight.traffic_risk)),
            "news": max(local_signals["news"], normalize_score(insight.news_risk)),
            "weather": max(local_signals["weather"], normalize_score(insight.weather_risk)),
        }

        adjustment = self._adjuster.adjust(
            profile.base_weights,
            request.ml_predictions,
            insight.weight_hints,
            feedback_bias=calibration.risk_bias,
        )
        weights = adjustment.profile

        route_modifiers = self._route_modifiers(request, route_distance, route_time)
        context_severity = max(signal_values["traffic"], signal_values["news"], signal_values["weather"])
        route_length_score = min(1.0, route_distance / 500.0)
        alternative_gap_score = self._alternative_gap_score(request, route_time)

        risk_score = clamp(
            100.0
            * (
                weights.traffic * signal_values["traffic"]
                + weights.news * signal_values["news"]
                + weights.weather * signal_values["weather"]
            )
            + context_severity * 14.0
            + route_length_score * 8.0
            + route_modifiers["risk_bonus"]
            + insight.risk_score * 0.25,
            0.0,
            100.0,
        )
        risk_score = clamp(risk_score * calibration.time_bias + calibration.risk_bias * 100.0, 0.0, 100.0)

        confidence_score = clamp(
            100.0
            - risk_score
            + insight.confidence_score * 0.35
            + alternative_gap_score * 18.0
            + route_modifiers["confidence_bonus"]
            + calibration.confidence_bias
            + profile.confidence_bias * 100.0,
            0.0,
            100.0,
        )

        explanation = self._build_explanation(
            request=request,
            insight=insight,
            profile=profile,
            adjustment=adjustment.summary,
            risk_score=risk_score,
            confidence_score=confidence_score,
            route_modifiers=route_modifiers,
            context_severity=context_severity,
            alternative_gap_score=alternative_gap_score,
            calibration=calibration,
        )

        return AnalysisResponse(
            risk_score=round(risk_score, 1),
            confidence_score=round(confidence_score, 1),
            explanation=explanation,
        )

    def _collect_local_signals(self, request: AnalysisRequest) -> dict[str, float]:
        if not request.route.steps:
            return {"traffic": 0.0, "news": 0.0, "weather": 0.0}

        totals = {"traffic": 0.0, "news": 0.0, "weather": 0.0}
        total_weight = 0.0
        for step in request.route.steps:
            factor = request.context.edge_factors.get(step.edge_id)
            if factor is None:
                continue
            weight = self._step_weight(step)
            totals["traffic"] += severity(factor.traffic_factor) * weight
            totals["news"] += severity(factor.news_factor) * weight
            totals["weather"] += severity(factor.weather_factor) * weight
            total_weight += weight

        if total_weight <= 0:
            return {"traffic": 0.0, "news": 0.0, "weather": 0.0}

        return {
            "traffic": min(1.0, totals["traffic"] / total_weight),
            "news": min(1.0, totals["news"] / total_weight),
            "weather": min(1.0, totals["weather"] / total_weight),
        }

    def _route_modifiers(self, request: AnalysisRequest, route_distance: float, route_time: float) -> dict[str, float]:
        modifiers = {"risk_bonus": 0.0, "confidence_bonus": 0.0}
        if route_distance > 0:
            modifiers["risk_bonus"] += min(10.0, route_distance * 0.15)
        if route_time > 0:
            modifiers["risk_bonus"] += min(12.0, route_time * 0.10)

        if request.ml_predictions:
            if request.ml_predictions.delay_probability is not None:
                modifiers["risk_bonus"] += normalize_score(request.ml_predictions.delay_probability) * 15.0
            if request.ml_predictions.route_reliability is not None:
                modifiers["confidence_bonus"] += normalize_score(request.ml_predictions.route_reliability) * 18.0
            if request.ml_predictions.confidence_delta is not None:
                modifiers["confidence_bonus"] += request.ml_predictions.confidence_delta

        if request.alternatives:
            best_alt_time = min((alt.total_time for alt in request.alternatives if alt.total_time > 0), default=0.0)
            best_alt_cost = min((alt.total_cost for alt in request.alternatives if alt.total_cost > 0), default=0.0)
            if best_alt_time > 0 and route_time > 0:
                time_saved = max(0.0, best_alt_time - route_time)
                modifiers["confidence_bonus"] += min(15.0, time_saved * 0.6)
            if best_alt_cost > 0 and request.route.total_cost > 0:
                cost_saved = max(0.0, best_alt_cost - request.route.total_cost)
                modifiers["confidence_bonus"] += min(8.0, cost_saved * 0.4)

        return modifiers

    def _build_explanation(
        self,
        request: AnalysisRequest,
        insight: InsightReport,
        profile: EndpointProfile,
        adjustment: str,
        risk_score: float,
        confidence_score: float,
        route_modifiers: dict[str, float],
        context_severity: float,
        alternative_gap_score: float,
        calibration: FeedbackCalibration,
    ) -> str:
        location = self._location_label(request)
        mode_label = {
            "news": "news disruption",
            "traffic": "traffic trend",
            "confidence": "route confidence",
        }[profile.name]
        route_phrase = self._route_phrase(request.route)

        parts = [f"{mode_label.title()} for {location}: {insight.explanation.strip() or 'No live insight was returned.'}"]
        if insight.key_events:
            parts.append("Key events: " + ", ".join(insight.key_events[:3]) + ".")
        parts.append(adjustment)
        parts.append(
            f"Risk sits at {risk_score:.1f}/100 and confidence at {confidence_score:.1f}/100 for {route_phrase}."
        )
        parts.append(
            f"Context severity is {context_severity * 100.0:.0f}/100 with an alternative gap score of {alternative_gap_score * 100.0:.0f}/100."
        )
        if route_modifiers["confidence_bonus"] > 0:
            parts.append(
                f"Route alternatives and ML predictions raised confidence by {route_modifiers['confidence_bonus']:.1f} points."
            )
        if calibration.samples > 0:
            parts.append(
                f"Feedback calibration is based on {calibration.samples} completed routes."
            )
        return " ".join(parts)

    def _location_label(self, request: AnalysisRequest) -> str:
        if request.location:
            return request.location.strip()
        if request.context.location_name:
            return request.context.location_name.strip()
        return "the selected route"

    def _route_phrase(self, route: Route) -> str:
        if route.route_id:
            return f"route {route.route_id}"
        if not route.steps:
            return "the route"
        first = self._node_label(route.steps[0], start=True)
        last = self._node_label(route.steps[-1], start=False)
        return f"{first} -> {last}" if first and last else "the route"

    def _node_label(self, step: RouteStep, start: bool) -> str:
        return (step.from_node_id if start else step.to_node_id) or step.edge_id or ""

    def _compute_total_distance(self, route: Route) -> float:
        return sum(step.distance for step in route.steps if step.distance > 0)

    def _compute_total_time(self, route: Route) -> float:
        return sum(step.time for step in route.steps if step.time > 0)

    def _step_weight(self, step: RouteStep) -> float:
        if step.time > 0:
            return step.time
        if step.distance > 0:
            return step.distance
        return 1.0

    def _alternative_gap_score(self, request: AnalysisRequest, route_time: float) -> float:
        if not request.alternatives or route_time <= 0:
            return 0.0

        best_alt_time = min((alt.total_time for alt in request.alternatives if alt.total_time > 0), default=0.0)
        if best_alt_time <= 0:
            return 0.0

        gap = max(0.0, best_alt_time - route_time)
        return min(1.0, gap / max(route_time, 1.0))

    def _profile_news(self) -> EndpointProfile:
        return EndpointProfile(
            name="news",
            base_weights={"traffic": 0.20, "news": 0.60, "weather": 0.20},
            news_boost=0.60,
            traffic_boost=0.20,
            weather_boost=0.20,
            confidence_bias=0.04,
        )

    def _profile_traffic(self) -> EndpointProfile:
        return EndpointProfile(
            name="traffic",
            base_weights={"traffic": 0.65, "news": 0.15, "weather": 0.20},
            news_boost=0.15,
            traffic_boost=0.65,
            weather_boost=0.20,
            confidence_bias=0.08,
        )

    def _profile_confidence(self) -> EndpointProfile:
        return EndpointProfile(
            name="confidence",
            base_weights={"traffic": 0.34, "news": 0.33, "weather": 0.33},
            news_boost=0.33,
            traffic_boost=0.34,
            weather_boost=0.33,
            confidence_bias=0.15,
        )
