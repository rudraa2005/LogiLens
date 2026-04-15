from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Request

from app.config import Settings
from app.schemas import AnalysisRequest, AnalysisResponse
from app.services.service import AIService


def create_app(service: AIService | None = None) -> FastAPI:
    settings = Settings.from_env()
    ai_service = service or AIService(settings=settings)

    @asynccontextmanager
    async def lifespan(_: FastAPI):
        try:
            yield
        finally:
            await ai_service.aclose()

    app = FastAPI(title="LogiLens AI Service", version="1.0.0", lifespan=lifespan)

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/analyze-news", response_model=AnalysisResponse)
    async def analyze_news(payload: AnalysisRequest) -> AnalysisResponse:
        return await ai_service.analyze_news(payload)

    @app.post("/predict-traffic", response_model=AnalysisResponse)
    async def predict_traffic(payload: AnalysisRequest) -> AnalysisResponse:
        return await ai_service.predict_traffic(payload)

    @app.post("/route-confidence", response_model=AnalysisResponse)
    async def route_confidence(payload: AnalysisRequest) -> AnalysisResponse:
        return await ai_service.route_confidence(payload)

    return app


app = create_app()

