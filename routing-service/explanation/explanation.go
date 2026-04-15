package explanation

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/rudraa2005/LogiLens/routing-service/comparison"
	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type routeEvidence struct {
	Traffic float64
	Weather float64
	News    float64
}

type routeIssue struct {
	Label    string
	Evidence routeEvidence
	Score    float64
}

func ExplainRoute(best comparison.Route, alternatives []comparison.Route, ctx rctx.Context) string {
	issues := collectRouteIssues(best, alternatives, ctx)
	parts := []string{}

	if len(issues) > 0 {
		locationPhrases := make([]string, 0, len(issues))
		for _, issue := range issues {
			reasons := make([]string, 0, 3)
			if phrase := severityPhrase("traffic", issue.Evidence.Traffic); phrase != "" {
				reasons = append(reasons, phrase)
			}
			if phrase := severityPhrase("weather", issue.Evidence.Weather); phrase != "" {
				reasons = append(reasons, phrase)
			}
			if phrase := severityPhrase("news", issue.Evidence.News); phrase != "" {
				reasons = append(reasons, phrase)
			}
			if len(reasons) == 0 {
				continue
			}
			locationPhrases = append(locationPhrases, fmt.Sprintf("%s near %s", joinPhrases(reasons), issue.Label))
		}

		if len(locationPhrases) > 0 {
			parts = append(parts, "This route avoids "+joinPhrases(locationPhrases))
		}
	}

	timeSaved, costSaved := savingsAgainstFastestAlternative(best, alternatives)
	if timeSaved > 0 {
		parts = append(parts, "saving "+formatFloat(timeSaved)+" minutes compared to the fastest alternative")
	}
	if costSaved > 0 {
		parts = append(parts, "reducing cost by "+formatFloat(costSaved))
	}

	if len(parts) == 0 {
		return "This route is the best available option based on the current route context."
	}

	return strings.Join(parts, ", ") + "."
}

func collectRouteIssues(best comparison.Route, alternatives []comparison.Route, ctx rctx.Context) []routeIssue {
	bestEdges := make(map[string]struct{}, len(best.Steps))
	for _, step := range best.Steps {
		if step.EdgeID == "" {
			continue
		}
		bestEdges[step.EdgeID] = struct{}{}
	}

	issuesByLabel := make(map[string]*routeEvidence)
	seenEdge := make(map[string]struct{})

	for _, alt := range alternatives {
		for _, step := range alt.Steps {
			if step.EdgeID == "" {
				continue
			}
			if _, ok := bestEdges[step.EdgeID]; ok {
				continue
			}
			if _, ok := seenEdge[step.EdgeID]; ok {
				continue
			}
			seenEdge[step.EdgeID] = struct{}{}

			factors, ok := ctx.EdgeFactors[step.EdgeID]
			if !ok {
				continue
			}

			label := routeLabel(step)
			if label == "" {
				label = "that corridor"
			}

			evidence := issuesByLabel[label]
			if evidence == nil {
				evidence = &routeEvidence{}
				issuesByLabel[label] = evidence
			}

			evidence.Traffic = math.Max(evidence.Traffic, factors.TrafficFactor)
			evidence.Weather = math.Max(evidence.Weather, factors.WeatherFactor)
			evidence.News = math.Max(evidence.News, factors.NewsFactor)
		}
	}

	issues := make([]routeIssue, 0, len(issuesByLabel))
	for label, evidence := range issuesByLabel {
		score := math.Max(evidence.Traffic, math.Max(evidence.Weather, evidence.News))
		if score < 1.2 {
			continue
		}
		issues = append(issues, routeIssue{
			Label:    label,
			Evidence: *evidence,
			Score:    score,
		})
	}

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Score != issues[j].Score {
			return issues[i].Score > issues[j].Score
		}
		return issues[i].Label < issues[j].Label
	})

	if len(issues) > 2 {
		issues = issues[:2]
	}

	return issues
}

func savingsAgainstFastestAlternative(best comparison.Route, alternatives []comparison.Route) (float64, float64) {
	var fastestTime float64
	var cheapestCost float64
	found := false

	for _, alt := range alternatives {
		if !found || alt.TotalTime < fastestTime {
			fastestTime = alt.TotalTime
		}
		if !found || alt.TotalCost < cheapestCost {
			cheapestCost = alt.TotalCost
		}
		found = true
	}

	if !found {
		return 0, 0
	}

	timeSaved := fastestTime - best.TotalTime
	costSaved := cheapestCost - best.TotalCost
	if timeSaved < 0 {
		timeSaved = 0
	}
	if costSaved < 0 {
		costSaved = 0
	}

	return timeSaved, costSaved
}

func routeLabel(step models.RouteStep) string {
	for _, value := range []string{step.ToNodeID, step.FromNodeID, step.EdgeID} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func severityPhrase(kind string, factor float64) string {
	switch kind {
	case "traffic":
		switch {
		case factor >= 1.8:
			return "heavy congestion"
		case factor >= 1.35:
			return "congestion"
		}
	case "weather":
		switch {
		case factor >= 1.8:
			return "severe weather"
		case factor >= 1.35:
			return "bad weather"
		}
	case "news":
		switch {
		case factor >= 1.8:
			return "major disruption"
		case factor >= 1.35:
			return "local disruption"
		}
	}
	return ""
}

func joinPhrases(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func formatFloat(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f", math.Round(value))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}
