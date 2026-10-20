package evaluation

import (
	"encoding/json"
	"os"
)

type EvaluationQuestion struct {
	Text           string `json:"text"`
	Type           string `json:"type"`
	ExpectedResult string `json:"expectedResult"`
}

type OverlapEvaluationInput struct {
	Questions  []EvaluationQuestion `json:"questions"`
	OutputDir  string               `json:"outputDir"`
	Overlaps   []int64              `json:"overlaps"`
	Step       int64                `json:"step"`
	Threshold  float32              `json:"threshold"`
	Type       string               `json:"type"`
	VectorSize uint64               `json:"vectorSize"`
}

func NewOverlapEvaluationInput(filename string) OverlapEvaluationInput {

	fileRead, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	var evaluationInput OverlapEvaluationInput
	if err := json.Unmarshal(fileRead, &evaluationInput); err != nil {
		panic(err)
	}
	return evaluationInput
}
