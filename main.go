package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()
	// Access your API key as an environment variable (see "Set up your API key" above)
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// The Gemini 1.5 models are versatile and work with both text-only and multimodal prompts
	model := client.GenerativeModel("gemini-1.5-pro")


	for {
		processReceipt(model, filename)
	}
}

func processReceipt(model genai.Model, filename string) 
	imgData1, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	prompt := []genai.Part{
		genai.ImageData("jpeg", imgData1),
		genai.Text(`What's the date, the total amount, the shop and a brief description of the purchased articles of this receipt? Format the output as JSON, example: {"date": "2023-10-27", "amount": "12.34", "shop": "Aldi", "description": ""}.`),
	}
	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		log.Fatal(err)
	}

	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				fmt.Println(part)
			}
		}
	}
}
