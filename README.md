# LogiLens
LogiLens – AI + Blockchain powered supply chain platform that predicts demand, detects risks, optimizes logistics, and ensures transparency. Features smart contracts, real-time dashboards, and scalable cloud-native architecture for global operations.

## Services
- `api-gateway-service`
- `shipment-service`
- `routing-service`
- `ai-service` for Groq-backed news analysis, traffic trend prediction, and route confidence scoring

Routing service AI gateway env:
- `AI_SERVICE_URL` defaults to `http://127.0.0.1:8085`
- `AI_SERVICE_TIMEOUT_SECONDS` defaults to `5`
- If the AI service is unavailable, routing still returns a local fallback insight payload
