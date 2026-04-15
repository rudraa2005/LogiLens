from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict, Field


class EdgeContext(BaseModel):
    traffic_factor: float = Field(default=1.0, ge=0.0)
    weather_factor: float = Field(default=1.0, ge=0.0)
    news_factor: float = Field(default=1.0, ge=0.0)


class Edge(BaseModel):
    model_config = ConfigDict(populate_by_name=True, extra="ignore")

    id: str
    from_node: str | None = Field(default=None, alias="from")
    to_node: str | None = Field(default=None, alias="to")
    mode_id: str | None = None
    distance: float = 0.0
    time: float = 0.0
    cost: float = 0.0


class RouteStep(BaseModel):
    model_config = ConfigDict(populate_by_name=True, extra="ignore")

    edge_id: str = ""
    from_node_id: str | None = None
    to_node_id: str | None = None
    mode_id: str | None = None
    distance: float = 0.0
    time: float = 0.0
    cost: float = 0.0


class Route(BaseModel):
    model_config = ConfigDict(extra="ignore")

    route_id: str | None = None
    steps: list[RouteStep] = Field(default_factory=list)
    total_distance: float = 0.0
    total_time: float = 0.0
    total_cost: float = 0.0


class Context(BaseModel):
    model_config = ConfigDict(extra="ignore")

    location_name: str | None = None
    edge_factors: dict[str, EdgeContext] = Field(default_factory=dict)


class MLPredictions(BaseModel):
    model_config = ConfigDict(extra="allow")

    traffic_trend: float | None = None
    news_risk: float | None = None
    weather_risk: float | None = None
    route_reliability: float | None = None
    delay_probability: float | None = None
    confidence_delta: float | None = None


class AnalysisRequest(BaseModel):
    model_config = ConfigDict(extra="ignore")

    location: str | None = None
    region: str | None = None
    time_bucket: str | None = None
    edges: list[Edge] = Field(default_factory=list)
    context: Context = Field(default_factory=Context)
    route: Route
    alternatives: list[Route] = Field(default_factory=list)
    ml_predictions: MLPredictions | None = None


class AnalysisResponse(BaseModel):
    risk_score: float
    confidence_score: float
    explanation: str


class RouteOutcomeFeedback(BaseModel):
    route_id: str | None = None
    predicted_time: float = Field(default=0.0, ge=0.0)
    actual_time: float = Field(default=0.0, ge=0.0)
    predicted_risk: float = Field(default=0.0, ge=0.0, le=100.0)
    actual_delay: float = Field(default=0.0, ge=0.0)


class FeedbackRequest(BaseModel):
    model_config = ConfigDict(extra="ignore")

    location: str | None = None
    context: Context = Field(default_factory=Context)
    route: Route
    feedback: RouteOutcomeFeedback


class FeedbackResponse(BaseModel):
    status: str
    samples: int
    time_bias: float
    risk_bias: float
    confidence_bias: float


class EdgeFactorResponse(BaseModel):
    edge_factors: dict[str, float] = Field(default_factory=dict)


class InsightReport(BaseModel):
    risk_score: float = 0.0
    confidence_score: float = 50.0
    explanation: str = ""
    traffic_risk: float | None = None
    news_risk: float | None = None
    weather_risk: float | None = None
    weight_hints: dict[str, float] = Field(default_factory=dict)
    key_events: list[str] = Field(default_factory=list)
    sources: list[str] = Field(default_factory=list)


class WeightProfile(BaseModel):
    traffic: float
    news: float
    weather: float
