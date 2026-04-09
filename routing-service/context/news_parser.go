package context

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type NewsSource interface {
	Fetch(query string) ([]string, error)
}

type NewsAnalyzer interface {
	Analyze(headlines []string) (float64, error)
}

type GoogleNewsSource struct {
	Client *http.Client
}

func NewGoogleNewsSource() *GoogleNewsSource {
	return &GoogleNewsSource{
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type OpenAINewsAnalyzer struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewOpenAINewsAnalyzer() NewsAnalyzer {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return KeywordNewsAnalyzer{}
	}

	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &OpenAINewsAnalyzer{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 20 * time.Second},
	}
}

type googleNewsRSS struct {
	Channel struct {
		Items []struct {
			Title string `xml:"title"`
		} `xml:"item"`
	} `xml:"channel"`
}

func (s *GoogleNewsSource) Fetch(query string) ([]string, error) {
	if strings.TrimSpace(query) == "" {
		return []string{}, nil
	}

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	endpoint := "https://news.google.com/rss/search?q=" + url.QueryEscape(query)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("news lookup failed: http %d", resp.StatusCode)
	}

	var feed googleNewsRSS
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	headlines := make([]string, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		if strings.TrimSpace(item.Title) != "" {
			headlines = append(headlines, item.Title)
		}
	}

	return headlines, nil
}

type KeywordNewsAnalyzer struct{}

func (KeywordNewsAnalyzer) Analyze(headlines []string) (float64, error) {
	if len(headlines) == 0 {
		return 1.0, nil
	}

	score := 1.0
	for _, headline := range headlines {
		text := strings.ToLower(headline)
		switch {
		case strings.Contains(text, "strike") || strings.Contains(text, "protest") || strings.Contains(text, "shutdown"):
			score = maxFloat(score, 1.8)
		case strings.Contains(text, "accident") || strings.Contains(text, "delay") || strings.Contains(text, "blocked"):
			score = maxFloat(score, 1.5)
		case strings.Contains(text, "traffic") || strings.Contains(text, "weather") || strings.Contains(text, "warning"):
			score = maxFloat(score, 1.2)
		}
	}

	if score > 2.0 {
		score = 2.0
	}

	return score, nil
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (a *OpenAINewsAnalyzer) Analyze(headlines []string) (float64, error) {
	if a == nil || strings.TrimSpace(a.APIKey) == "" {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}

	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	model := strings.TrimSpace(a.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You score regional logistics disruption risk from news headlines. Return only JSON like {\"factor\": 1.0}. Use 1.0 for no impact, 1.2 for mild disruption, 1.5 for moderate disruption, 1.8 for severe disruption, and 2.0 for extreme disruption.",
			},
			{
				"role":    "user",
				"content": "Headlines:\n" + strings.Join(headlines, "\n"),
			},
		},
		"temperature": 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}
	if len(chatResp.Choices) == 0 {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if content == "" {
		return KeywordNewsAnalyzer{}.Analyze(headlines)
	}

	var parsed struct {
		Factor float64 `json:"factor"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err == nil && parsed.Factor >= 1.0 {
		if parsed.Factor > 2.0 {
			parsed.Factor = 2.0
		}
		return parsed.Factor, nil
	}

	var factor float64
	if _, err := fmt.Sscanf(content, "%f", &factor); err == nil && factor >= 1.0 {
		if factor > 2.0 {
			factor = 2.0
		}
		return factor, nil
	}

	return KeywordNewsAnalyzer{}.Analyze(headlines)
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
