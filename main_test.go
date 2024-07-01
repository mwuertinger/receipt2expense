package main

import "testing"

func TestExtractJson(t *testing.T) {
	sampleOutput := "```json\n{\"date\": \"2023-07-01\", \"amount\": \"34.15\", \"shop\": \"Mary's Apotheke Südseite\", \"description\": \"Omeprazol, Artelac Lipids, Posiforlid Augenspray\", confidence: \"1.0\"}\n```"
	expected := "{\"date\": \"2023-07-01\", \"amount\": \"34.15\", \"shop\": \"Mary's Apotheke Südseite\", \"description\": \"Omeprazol, Artelac Lipids, Posiforlid Augenspray\", confidence: \"1.0\"}"
	got := extractJson(sampleOutput)
	if got != expected {
		t.Errorf("expected: %s\n                  got: %s", expected, got)
	}
}
