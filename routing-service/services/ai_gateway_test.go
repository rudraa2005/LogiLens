package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestAIGatewayReturnsInsightsFromService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/route-confidence" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"risk_score":       81.4,
			"confidence_score": 63.2,
			"explanation":      "AI-backed route analysis completed.",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	gateway := &AIGateway{
		baseURL: server.URL,
		client:  server.Client(),
	}

	insights, err := gateway.GetRouteInsights(context.Background(), testRouteResponse(), testRouteContext())
	if err != nil {
		t.Fatalf("expected successful call, got error: %v", err)
	}
	if insights == nil || !insights.Available || insights.Fallback {
		t.Fatalf("expected live insights, got %+v", insights)
	}
	if insights.RiskScore != 81.4 || insights.ConfidenceScore != 63.2 {
		t.Fatalf("unexpected scores: %+v", insights)
	}
}

func TestAIGatewayFallsBackOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"risk_score":       70,
			"confidence_score": 40,
			"explanation":      "too slow",
		})
	}))
	defer server.Close()

	gateway := &AIGateway{
		baseURL: server.URL,
		client: &http.Client{
			Timeout: 20 * time.Millisecond,
		},
	}

	insights, err := gateway.GetRouteInsights(context.Background(), testRouteResponse(), testRouteContext())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if insights == nil || !insights.Fallback || insights.Available {
		t.Fatalf("expected fallback insights, got %+v", insights)
	}
	if insights.Explanation == "" {
		t.Fatal("expected fallback explanation")
	}
}

func testRouteResponse() RouteResponse {
	return RouteResponse{
		SourceNodeID:      "source-node",
		DestinationNodeID: "dest-node",
		RouteID:           "route-1",
		Steps: []models.RouteStep{
			{
				EdgeID:     "edge-1",
				FromNodeID: "source-node",
				ToNodeID:   "dest-node",
				Distance:   10,
				Time:       12,
				Cost:       5,
			},
		},
		TotalDistance: 10,
		TotalTime:     12,
		TotalCost:     5,
	}
}

func testRouteContext() rctx.Context {
	return rctx.Context{
		EdgeFactors: map[string]rctx.EdgeContext{
			"edge-1": {TrafficFactor: 1.5, WeatherFactor: 1.1, NewsFactor: 1.2},
		},
	}
}
