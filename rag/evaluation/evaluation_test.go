package evaluation

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type StepAndVectorSize struct {
	Vsize uint64
	Step  int64
}

func parseStepAndVsize(vsize string, step string) StepAndVectorSize {
	valv, err := strconv.ParseUint(vsize, 10, 64)
	vals, err := strconv.ParseInt(step, 10, 64)

	if err != nil {
		panic(err)
	}

	return StepAndVectorSize{Vsize: valv, Step: vals}
}

func TestEvaluationOverlapLoadDatabase(t *testing.T) {
	overlaps := []int64{25, 50, 100, 150, 200, 250, 500}
	step_env := os.Getenv("RAG_TEST_STEP")
	vsize_env := os.Getenv("RAG_TEST_VSIZE")
	sv := parseStepAndVsize(vsize_env, step_env)
	step := sv.Step
	vsize := sv.Vsize
	
	filepaths := os.Getenv("RAG_TEST_FILEPATHS")
	filepathsSlice := strings.Split(filepaths, ",")
	evaluationService := CreateOverlapEvaluation(
		overlaps,
		step,
		vsize,
		[]EvaluationQuestion{},
		filepathsSlice,
	)
	evaluationService.Load()
}

func TestEvaluationOverlap(t *testing.T) {
	filename := "../../eval/evaluation_overlap.json"
	input := NewOverlapEvaluationInput(filename)
	fmt.Println(input)
	evaluationService := CreateOverlapEvaluation(
		input.Overlaps,
		input.Step,
		input.VectorSize,
		input.Questions,
		[]string{},
	)
	analysis, err := evaluationService.Process(&input)
	if err != nil {
		panic(err)
	}

	result, _ := json.Marshal(analysis)

	filename = fmt.Sprintf("%s%s_OVERLAPS_result.json", input.OutputDir ,uuid.NewMD5(uuid.NameSpaceURL, result))
	if err := os.WriteFile(filename, result, os.FileMode(os.O_WRONLY)); err != nil {
		fmt.Printf("Error is %v", err)
	}

}
