from __future__ import annotations

import unittest

from fastapi.testclient import TestClient

from app.main import create_app
from app.schemas import AnalysisRequest, AnalysisResponse, InsightReport, MLPredictions
from app.services.service import AIService


class StubInsightClient:
    def __init__(self) -> None:
        self.calls: list[str] = []

    async def analyze(self, mode: str, request: AnalysisRequest, route_summary: str) -> InsightReport:
        self.calls.append(mode)
        if mode == "news":
            return InsightReport(
                risk_score=72.0,
                confidence_score=58.0,
                explanation="Recent reports show a disruption cluster near the corridor.",
                traffic_risk=31.0,
                news_risk=83.0,
                weather_risk=24.0,
                weight_hints={"traffic": 0.9, "news": 1.4, "weather": 0.8},
                key_events=["road closure", "public event"],
            )
        if mode == "traffic":
            return InsightReport(
                risk_score=63.0,
                confidence_score=61.0,
                explanation="Traffic is building around the downtown approach during peak hours.",
                traffic_risk=88.0,
                news_risk=34.0,
                weather_risk=22.0,
                weight_hints={"traffic": 1.5, "news": 0.8, "weather": 0.9},
                key_events=["peak hour congestion"],
            )
        return InsightReport(
            risk_score=41.0,
            confidence_score=72.0,
            explanation="Route conditions look manageable with moderate volatility.",
            traffic_risk=45.0,
            news_risk=37.0,
            weather_risk=26.0,
            weight_hints={"traffic": 1.0, "news": 1.0, "weather": 1.0},
            key_events=["moderate delay risk"],
        )

    async def aclose(self) -> None:  # pragma: no cover - nothing to close
        return None


def sample_request() -> dict:
    return {
        "location": "Bengaluru, India",
        "edges": [
            {"id": "e-1", "from": "A", "to": "B", "distance": 4, "time": 8, "cost": 2},
            {"id": "e-2", "from": "B", "to": "C", "distance": 6, "time": 10, "cost": 3},
        ],
        "context": {
            "location_name": "Bengaluru",
            "edge_factors": {
                "e-1": {"traffic_factor": 1.4, "weather_factor": 1.1, "news_factor": 1.8},
                "e-2": {"traffic_factor": 1.7, "weather_factor": 1.0, "news_factor": 1.1},
            },
        },
        "route": {
            "route_id": "route-1",
            "steps": [
                {"edge_id": "e-1", "from_node_id": "A", "to_node_id": "B", "distance": 4, "time": 8, "cost": 2},
                {"edge_id": "e-2", "from_node_id": "B", "to_node_id": "C", "distance": 6, "time": 10, "cost": 3},
            ],
            "total_distance": 10,
            "total_time": 18,
            "total_cost": 5,
        },
        "alternatives": [
            {
                "route_id": "alt-1",
                "steps": [
                    {"edge_id": "e-1", "from_node_id": "A", "to_node_id": "B", "distance": 4, "time": 12, "cost": 4},
                    {"edge_id": "e-2", "from_node_id": "B", "to_node_id": "C", "distance": 6, "time": 13, "cost": 5},
                ],
                "total_distance": 10,
                "total_time": 25,
                "total_cost": 9,
            }
        ],
        "ml_predictions": {
            "traffic_trend": 0.82,
            "news_risk": 0.74,
            "weather_risk": 0.25,
            "route_reliability": 0.6,
            "delay_probability": 0.7,
            "confidence_delta": 2.5,
        },
    }


class TestAIServiceApp(unittest.TestCase):
    def setUp(self) -> None:
        self.insight_client = StubInsightClient()
        self.app = create_app(service=AIService(insight_client=self.insight_client))
        self.client = TestClient(self.app)

    def test_health(self) -> None:
        response = self.client.get("/health")
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["status"], "ok")

    def test_analyze_news(self) -> None:
        response = self.client.post("/analyze-news", json=sample_request())
        self.assertEqual(response.status_code, 200)
        body = AnalysisResponse.model_validate(response.json())
        self.assertGreater(body.risk_score, 0)
        self.assertIn("news", body.explanation.lower())

    def test_predict_traffic(self) -> None:
        response = self.client.post("/predict-traffic", json=sample_request())
        self.assertEqual(response.status_code, 200)
        body = AnalysisResponse.model_validate(response.json())
        self.assertGreater(body.risk_score, 0)
        self.assertTrue("traffic" in body.explanation.lower() or "congestion" in body.explanation.lower())

    def test_route_confidence(self) -> None:
        response = self.client.post("/route-confidence", json=sample_request())
        self.assertEqual(response.status_code, 200)
        body = AnalysisResponse.model_validate(response.json())
        self.assertGreater(body.confidence_score, 0)
        self.assertIn("ml adjusted weights", body.explanation.lower())


if __name__ == "__main__":  # pragma: no cover
    unittest.main()
