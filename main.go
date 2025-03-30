package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/option"
)

var parameters = map[string]*genai.Schema{
	"date": {
		Type:        genai.TypeString,
		Description: "Receipt date in ISO8601 format, eg. 2024-02-17",
	},
	"amount": {
		Type:        genai.TypeNumber,
		Description: "Total amount of the receipt.",
	},
	"shop": {
		Type:        genai.TypeString,
		Description: "Shop where the purchase took place.",
	},
	"description": {
		Type:        genai.TypeString,
		Description: "Brief description of the purchased articles.",
	},
}
var requiredParameters = []string{"date", "amount", "shop", "description"}

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
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-pro")

	model.Tools = []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{{
				Name:        "addReceipt",
				Description: "Add a new receipt.",
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: parameters,
					Required:   requiredParameters,
				},
			}},
		},
	}

	// Serve static files (HTML, JS, CSS, etc.)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	http.HandleFunc("/receipt", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-Type") != "image/jpeg" {
			fmt.Fprintf(w, "jpeg image required, got: %s", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		jpegImage, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintf(w, "failed to read body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		expense, err := processReceipt(ctx, model, jpegImage)
		if err != nil {
			log.Printf("gemini error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		jsonStr, err := json.Marshal(expense)
		if err != nil {
			log.Printf("failed to marshal expense: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonStr); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	})

	err = http.ListenAndServeTLS("[::]:8080", "cert.pem", "key.pem", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

const prompt = `Parse this receipt and pass the data to the addReceipt function.`

func processReceipt(ctx context.Context, model *genai.GenerativeModel, jpegImage []byte) (*expense, error) {
	prompt := []genai.Part{
		genai.ImageData("jpeg", jpegImage),
		genai.Text(prompt),
	}
	var resp *genai.GenerateContentResponse
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		resp, err = model.GenerateContent(ctx, prompt...)
		if err == nil {
			break
		}
		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) {
			status := apiErr.HTTPCode()
			if status == 500 || status == 503 && i < maxRetries-1 {
				delay := time.Duration(1+rand.Intn(2<<i)) * time.Second
				log.Printf("Gemimi returned status %d, retrying after %v", apiErr.HTTPCode(), delay)
				time.Sleep(delay)
				continue
			}
		}
		return nil, fmt.Errorf("generateContent: %v", err)
	}
	if len(resp.Candidates) != 1 {
		return nil, fmt.Errorf("expected 1 candidate, got: %d", len(resp.Candidates))
	}
	candidate := resp.Candidates[0]
	if len(candidate.Content.Parts) != 1 {
		return nil, fmt.Errorf("expected 1 part, got: %d", len(candidate.Content.Parts))
	}
	call, ok := candidate.Content.Parts[0].(genai.FunctionCall)
	if !ok {
		return nil, fmt.Errorf("expected FunctionCall, got: %T", candidate.Content.Parts[0])
	}
	if call.Name != "addReceipt" {
		return nil, fmt.Errorf("expected addReceipt, got: %s", call.Name)
	}
	args := call.Args

	for _, parameter := range requiredParameters {
		if _, ok := args[parameter]; !ok {
			return nil, fmt.Errorf("args (%v) is missing required parameter %s", args, parameter)
		}
		ok := false
		switch parameters[parameter].Type {
		case genai.TypeString:
			_, ok = args[parameter].(string)
		case genai.TypeNumber:
			_, ok = args[parameter].(float64)
		}
		if !ok {
			return nil, fmt.Errorf("parameter %s must be %v, got: %T", parameter, parameters[parameter].Type, args[parameter])
		}
	}

	return &expense{
		Date:        args["date"].(string),
		Amount:      args["amount"].(float64),
		Shop:        args["shop"].(string),
		Description: args["description"].(string),
	}, nil
}

func extractJson(text string) string {
	text = strings.TrimPrefix(text, "```json\n")
	text = strings.TrimSuffix(text, "\n```")
	return text
}

type expense struct {
	FileName    string  `json:"filename"`
	Date        string  `json:"date"`
	Amount      float64 `json:"amount"`
	Shop        string  `json:"shop"`
	Description string  `json:"description"`
}
