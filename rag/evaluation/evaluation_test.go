package evaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

// go test -timeout 30m
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
	// TODO: this is a good candidate for an env variable
	filename := "../../eval/evaluation_overlap.json"
	input := NewOverlapEvaluationInput(filename)
	fmt.Println(input)
	overlapInput, ok := input.(OverlapEvaluationInput)
	if !ok {
		t.Fatalf("Expected OverlapEvaluationInput, got %T", input)
	}
	evaluationService := CreateOverlapEvaluation(
		overlapInput.Overlaps,
		overlapInput.Step,
		overlapInput.VectorSize,
		overlapInput.Questions,
		[]string{},
	)

	analysis, err := evaluationService.Process(&overlapInput)
	if err != nil {
		panic(err)
	}

	result, _ := json.Marshal(analysis)

	filename = fmt.Sprintf("%s%s_OVERLAPS_result.json", overlapInput.OutputDir, uuid.NewMD5(uuid.NameSpaceURL, result))
	if err := os.WriteFile(filename, result, os.FileMode(os.O_WRONLY)); err != nil {
		fmt.Printf("Error is %v", err)
	}



	// maybe it's a good idea to split this into a function
	cmd := exec.Command("python3", "./python/plot.py", "-o", "../../output/files", "--overlap", "true")

	cmd.Stdin = bytes.NewReader(result)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Python script failed: %v\nStderr: %s", err, stderr.String())
	}

	fmt.Print(stdout.String())

}

// go test -timeout 30m
func TestEvaluationStepLoadDatabase(t *testing.T) {
	steps := []int64{
		256,
		512,
		1024,
		2048,
		4096,
	}

	overlap_env := os.Getenv("RAG_TEST_OVERLAP")
	vsize_env := os.Getenv("RAG_TEST_VSIZE")
	sv := parseStepAndVsize(vsize_env, overlap_env)
	overlap := sv.Step
	vsize := sv.Vsize

	filepaths := os.Getenv("RAG_TEST_FILEPATHS")
	filepathsSlice := strings.Split(filepaths, ",")
	evaluationService := CreateStepEvaluation(
		steps,
		overlap,
		vsize,
		[]EvaluationQuestion{},
		filepathsSlice,
	)
	evaluationService.Load()
}

func TestEvaluationStep(t *testing.T) {
	// TODO: this is a good candidate for an env variable
	filename := "../../eval/evaluation_step.json"
	input := NewStepEvaluationInput(filename)
	fmt.Println(input)
	stepInput, ok := input.(StepEvaluationInput)
	if !ok {
		t.Fatalf("Expected StepEvaluationInput, got %T", input)
	}
	evaluationService := CreateStepEvaluation(
		stepInput.Steps,
		stepInput.Overlap,
		stepInput.VectorSize,
		stepInput.Questions,
		[]string{},
	)

	analysis, err := evaluationService.Process(&stepInput)
	if err != nil {
		panic(err)
	}

	result, _ := json.Marshal(analysis)

	filename = fmt.Sprintf("%s%s_STEPS_result.json", stepInput.OutputDir, uuid.NewMD5(uuid.NameSpaceURL, result))
	if err := os.WriteFile(filename, result, os.FileMode(os.O_WRONLY)); err != nil {
		fmt.Printf("Error is %v", err)
	}

	// maybe it's a good idea to split this into a function
	cmd := exec.Command("python3", "./python/plot.py", "-o", "../../output/files")

	cmd.Stdin = bytes.NewReader(result)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Python script failed: %v\nStderr: %s", err, stderr.String())
	}

	fmt.Print(stdout.String())

}
