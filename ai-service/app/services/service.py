from __future__ import annotations

from app.config import Settings
from app.schemas import AnalysisRequest, AnalysisResponse
from app.services.groq_client import GroqInsightClient
from app.services.scoring import RouteScorer


class AIService:
    def __init__(self, settings: Settings | None = None, insight_client: GroqInsightClient | None = None) -> None:
        self.settings = settings or Settings.from_env()
        self._insight_client = insight_client or GroqInsightClient.create(self.settings)
        self._scorer = RouteScorer()

    async def aclose(self) -> None:
        close = getattr(self._insight_client, "aclose", None)
        if callable(close):
            await close()

    async def analyze_news(self, request: AnalysisRequest) -> AnalysisResponse:
        return await self._evaluate("news", request)

    async def predict_traffic(self, request: AnalysisRequest) -> AnalysisResponse:
        return await self._evaluate("traffic", request)

    async def route_confidence(self, request: AnalysisRequest) -> AnalysisResponse:
        return await self._evaluate("confidence", request)

    async def _evaluate(self, mode: str, request: AnalysisRequest) -> AnalysisResponse:
        insight = await self._insight_client.analyze(mode=mode, request=request, route_summary=self._route_summary(request))
        if mode == "news":
            return self._scorer.analyze_news(request, insight)
        if mode == "traffic":
            return self._scorer.predict_traffic(request, insight)
        return self._scorer.route_confidence(request, insight)

    def _route_summary(self, request: AnalysisRequest) -> str:
        if request.route.route_id:
            route_name = request.route.route_id
        elif request.location:
            route_name = request.location.strip()
        elif request.context.location_name:
            route_name = request.context.location_name.strip()
        else:
            route_name = "selected route"

        if not request.route.steps:
            return route_name

        parts: list[str] = []
        for step in request.route.steps[:5]:
            label = step.from_node_id or step.edge_id or "node"
            next_label = step.to_node_id or ""
            if next_label:
                parts.append(f"{label}->{next_label}")
            else:
                parts.append(label)

        if len(request.route.steps) > 5:
            parts.append("...")

        totals = []
        if request.route.total_distance > 0:
            totals.append(f"{request.route.total_distance:.1f} km")
        if request.route.total_time > 0:
            totals.append(f"{request.route.total_time:.1f} min")

        if totals:
            return f"{route_name} ({', '.join(parts)}; {' / '.join(totals)})"
        return f"{route_name} ({' -> '.join(parts)})"

