# Routing Service Progress Report

Generated on: 2026-04-10

## Executive Summary

The routing service is **mostly implemented at the core-functional level**. The system can:

- accept a `ComputeRoute` gRPC request,
- geocode the source and destination,
- build a graph from stored route edges,
- run A* pathfinding with traffic, weather, and news context,
- persist the route and route steps to PostgreSQL,
- and return a protobuf response with totals and step geometry.

**Overall progress estimate: ~80% complete.**

That number is an estimate based on the current codebase. It reflects that the main routing pipeline exists, but the service still needs testing, hardening, and some cleanup before it can be considered production-ready.

## What Is Done

### 1. Service entrypoint and gRPC wiring

- The service boots from [`routing-service/cmd/main.go`](./cmd/main.go).
- It connects to PostgreSQL, loads edges, builds the routing graph, and starts a gRPC server on `:50052`.
- The gRPC server is registered through the generated proto service interface.
'

### 2. Routing domain model

- Core models exist for nodes, edges, routes, route steps, transport modes, and coordinates in [`routing-service/models/routing.go`](./models/routing.go).
- The proto contract is defined in [`proto/route.proto`](../proto/route.proto) and exposes a single RPC:
  - `ComputeRoute(RouteRequest) returns (RouteResponse)`

### 3. Graph construction and pathfinding

- The graph builder is implemented in [`routing-service/graph/graph.go`](./graph/graph.go).
- The service supports nearest-node lookup and nearest-edge lookup in [`routing-service/graph/nearest.go`](./graph/nearest.go).
- A* search is implemented in [`routing-service/graph/astar.go`](./graph/astar.go).

### 4. Geocoding

- Place names are resolved to coordinates using a configurable [`GeoService`](./geoservice) with TomTom, OpenCage, or Google as the external provider.
- An in-memory cache keyed by normalized place names avoids repeat lookups.

### 5. Context-aware routing inputs

The service can enrich routing weights with live external signals:

- traffic incidents from TomTom in [`routing-service/context/traffic.go`](./context/traffic.go),
- weather from Open-Meteo in [`routing-service/context/weather.go`](./context/weather.go),
- news-based disruption scoring in [`routing-service/context/news_parser.go`](./context/news_parser.go).

The context builder combines those signals into edge factors that influence A* weight calculations.

### 6. Route persistence

- PostgreSQL access is implemented in [`routing-service/db/pool.go`](./db/pool.go).
- Route and step persistence is implemented in [`routing-service/repository/route_repo.go`](./repository/route_repo.go).
- Route creation stores the route header plus per-step records.

### 7. Service orchestration

- Business logic sits in [`routing-service/services/route_service.go`](./services/route_service.go).
- It ties together geocoding, graph search, context building, route persistence, and protobuf conversion.
- The gRPC handler in [`routing-service/server/route_server.go`](./server/route_server.go) forwards requests into the service layer.

## What Is Partially Done

### 1. Optimization request handling

- The proto request includes `allowed_modes`, but the current Go service path does not appear to enforce mode filtering yet.
- `optimize_by` is supported, but the conversion from proto enum to service string is currently very lightweight.

### 2. Data loading strategy

- The graph is built from edges only in `cmd/main.go`.
- Nodes are inferred from edge geometry if they are missing, which works for route execution but may not be the cleanest long-term data model.

### 3. Response/persistence alignment

- The service returns step geometry in the API response.
- The database step insert currently stores step metadata, but not the step geometry payload itself.

## Main Gaps

These are the biggest remaining items if the goal is a production-ready routing service:

1. **No tests yet**
   - There are no `*_test.go` files under `routing-service/`.
   - This is the largest gap because it leaves the pathfinding, geocoding, and persistence flow unverified.

2. **Limited validation and error handling**
   - Input validation is minimal.
   - There is little protection around invalid places, unsupported optimization modes, or partial upstream failures.

3. **Configuration hardening**
   - Some defaults are embedded in code, including DB and API defaults.
   - Secrets and runtime configuration should be fully externalized.

4. **Production readiness**
   - No health endpoint or readiness checks are visible in the routing service package.
   - No retry/backoff strategy is implemented for external API calls.
   - No caching policy or invalidation strategy is defined beyond the in-memory geocode cache.

5. **Operational documentation**
   - There is no routing-service-specific README describing deployment, required env vars, or database expectations.

## Status Breakdown

| Area | Status | Notes |
| --- | --- | --- |
| gRPC API wiring | Done | `ComputeRoute` is implemented end to end |
| Graph building | Done | Graph is built from DB edges and geometry |
| Pathfinding | Done | A* is implemented |
| Geocoding | Done | TomTom geocoding works with caching |
| Traffic context | Done | TomTom incident lookup implemented |
| Weather context | Done | Open-Meteo lookup implemented |
| News context | Done | RSS fetch plus OpenAI/keyword scoring implemented |
| Database persistence | Done | Routes and steps are inserted |
| Proto response conversion | Done | Maps service output to protobuf |
| Tests | Not done | No automated test coverage found |
| Input validation | Partial | Basic checks exist, but coverage is limited |
| Production hardening | Partial | Needs config, retries, docs, and ops work |

## Bottom Line

The routing service has the **core routing engine in place** and is beyond the prototype stage. The main workflow exists from request to response, including route computation and persistence. What remains is mostly **quality, correctness, and operational hardening** rather than core feature invention.

If you want a single-sentence summary:

**The routing service is roughly four-fifths complete, with the main pathfinding and context-aware routing logic done, but tests and production hardening still outstanding.**
