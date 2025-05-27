package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type Calculation struct {
	Operation string  `json:"operation"`
	A         float64 `json:"a"`
	B         float64 `json:"b"`
	Result    float64 `json:"result"`
}

func calculate(calc Calculation) float64 {
	switch calc.Operation {
	case "add":
		return calc.A + calc.B
	case "subtract":
		return calc.A - calc.B
	case "multiply":
		return calc.A * calc.B
	case "divide":
		if calc.B == 0 {
			return 0
		}
		return calc.A / calc.B
	default:
		return 0
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var calc Calculation
	if err := json.NewDecoder(r.Body).Decode(&calc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	calc.Result = calculate(calc)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(calc)
}

func main() {
	port := "8080"
	http.HandleFunc("/calculate", handler)
	log.Printf("Calculator server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
