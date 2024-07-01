package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"log"
	"os"
	"os/signal"
	"path"
	"strings"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		sig := <-sigc
		if sig == os.Interrupt {
			cancel()
		}
	}()

	// Access your API key as an environment variable (see "Set up your API key" above)
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")

	receiptDir := os.Args[1]
	entries, err := os.ReadDir(receiptDir)
	if err != nil {
		log.Fatalf("readdir: %v", err)
	}
	var expenses []*expense
	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		if entry.IsDir() {
			continue
		}
		expense, err := processReceipt(ctx, model, path.Join(receiptDir, entry.Name()))
		if err != nil {
			log.Printf("%s: %v", entry.Name(), err)
		}
		jsonStr, err := json.Marshal(expense)
		if err != nil {
			log.Printf("%s: %v", entry.Name(), err)
		}
		fmt.Println(string(jsonStr))

		expenses = append(expenses, expense)
	}
}

const prompt = `What's the date (ISO8601), the total amount, the shop and a brief description of the purchased articles of this receipt?
What is your confidence (as a float between 0 (very uncertain) and 1 (completely certain)) about the correctness of the output?
Format the output as JSON, example: {"date": "2023-10-27", "amount": "12.34", "shop": "Aldi", "description": "Groceries", "confidence": 0.8}.`

func processReceipt(ctx context.Context, model *genai.GenerativeModel, filename string) (*expense, error) {
	imgData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("readFile: %v", err)
	}

	prompt := []genai.Part{
		genai.ImageData("jpeg", imgData),
		genai.Text(prompt),
	}
	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, fmt.Errorf("generateContent: %v", err)
	}
	if len(resp.Candidates) != 1 {
		return nil, fmt.Errorf("expected 1 candidate, got: %d", len(resp.Candidates))
	}
	candidate := resp.Candidates[0]
	if len(candidate.Content.Parts) != 1 {
		return nil, fmt.Errorf("expected 1 part, got: %d", len(candidate.Content.Parts))
	}
	text, ok := candidate.Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("expected Text but got %T", text)
	}
	jsonStr := extractJson(string(text))
	expense := expense{FileName: filename}
	if err := json.Unmarshal([]byte(jsonStr), &expense); err != nil {
		return nil, fmt.Errorf("unmarshal(%s): %v", jsonStr, err)
	}
	return &expense, nil
}

func extractJson(text string) string {
	text = strings.TrimPrefix(text, "```json\n")
	text = strings.TrimSuffix(text, "\n```")
	return text
}

type expense struct {
	FileName    string  `json:"filename"`
	Date        string  `json:"date"`
	Amount      string  `json:"amount"`
	Shop        string  `json:"shop"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}
