import requests

OLLAMA_URL = "http://localhost:11434/api/generate"

payload = {
    "model": "qwen2.5:3b",
    "prompt": '''You are an information extraction system.

Extract structured data from the text below.

Return ONLY valid JSON.
Do NOT include explanations or additional text.
If a field is not explicitly stated, infer conservatively and lower confidence.

Schema:
{
  "event_type": string,
  "location": string,
  "severity": "low" | "medium" | "high",
  "time_window": string | null,
  "confidence": number
}

Text:
"Port congestion reported in Rotterdam due to labor strikes, expected to last several days."''',
    "stream": False
}

response = requests.post(OLLAMA_URL, json=payload)
response.raise_for_status()

print(response.json()["response"])