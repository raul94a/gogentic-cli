// Evaluation systems for the package RAG implementation
// the evaluation system must be run as part of a test of a CLI tool
package evaluation

import (
	"fmt"
	rag "gogentic-rag"
	"strconv"
	"strings"
	"sync"
)

type Evaluation interface {
	Load() error
	Process(EvaluationInput) ([]EvaluationOutput, error)
}

// EvaluationOverlapsImpl hold the parameters in order to evaluate different parameters for the RAG implementation
type EvaluationOverlapsImpl struct {
	Overlaps   []int64
	Step       int64
	VectorSize uint64
	questions  []EvaluationQuestion
	Filepaths  []string
}



func (e *EvaluationOverlapsImpl) Load() error {
	const COLLECTION_NAME = "evaluation_overlap_size_{size}"
	const THREASHOLD = 0.655
	overlaps := e.Overlaps
	step := e.Step
	vsize := e.VectorSize
	services := make([]rag.Rag, 0)
	for i := range overlaps {
		overlap := overlaps[i]

		collection := strings.ReplaceAll(COLLECTION_NAME, "{size}", strconv.Itoa(int(overlap)))
		svc := rag.NewRagService(collection, vsize, THREASHOLD, step, overlap)
		services = append(services, svc)
	}

	for _, svc := range services {

		var wait sync.WaitGroup
		var readErr error
		ragservice := svc.(*rag.RagService)
		wait.Go(func() {
			for _, file := range e.Filepaths {

				fmt.Printf("Proccessing file %s for Overlap %d\n", file, ragservice.GetOverlap())
				if err := svc.ProcessFile(file); err != nil {
					readErr = err
				}
			}
		})

		if readErr != nil {
			return readErr
		}

		// Write in the database should be done
		wait.Wait()
	}
	return nil
}

// Example of evaluation:
//
// Any other evaluation struct can inherit from the Evaluation interfaace
// and implement its own evaluation. Depending on the used source the developer may
// have to program its own implementation.
//
// We have N services ready to process the evaluation algorithm
// Each service will:
// 1. Load the information into Qdrant
// 2. Use the retrieval pipeline -Ask a question
// 3. Track the score for the Retrieval
//
// Process will take the K closes documents for the input. Check LIMIT to change it's value
// Maybe it's a good idea to pass the value to the function in order to configure it.
func (e *EvaluationOverlapsImpl) Process(input EvaluationInput) ([]EvaluationOutput, error) {

	const COLLECTION_NAME = "evaluation_overlap_size_{size}"
	const THREASHOLD = 0.655
	const LIMIT = 10
	overlaps := e.Overlaps
	step := e.Step
	vsize := e.VectorSize
	services := make([]rag.Rag, 0)

	evals := make([]EvaluationOutput, 0)

	questionTable := make(map[string]string)

	overlapInput, ok := (input).(*OverlapEvaluationInput)
	if !ok {
		// Always check 'ok' so your app doesn't panic if the type is wrong
		return make([]EvaluationOutput, 0),fmt.Errorf("expected *OverlapEvaluationInput, got %T", input)
	}

	for _, q := range overlapInput.Questions {
		questionTable[q.Text] = q.ExpectedResult
	}

	for i := range overlaps {
		overlap := overlaps[i]
		collection := strings.ReplaceAll(COLLECTION_NAME, "{size}", strconv.Itoa(int(overlap)))
		svc := rag.NewRagService(collection, vsize, THREASHOLD, step, overlap)
		services = append(services, svc)
	}

	for _, question := range e.questions {

		overlapEval := EvaluationOverlapOutput{
			Question: question.Text,
			Results:  make([]Result, 0),
		}

		for _, svc := range services {

			result, err := GetResult(question, &questionTable, svc, LIMIT)
			if err != nil {
				return []EvaluationOutput{}, err
			}
			if result == nil {
				panic("[Error] Result cannot be null")
			}
			overlapEval.Results = append(overlapEval.Results, *result)

		}

		evals = append(evals, overlapEval)

	}

	return evals, nil

}

func GetResult(question EvaluationQuestion, questionTable *map[string]string, svc rag.Rag, limit int) (*Result, error) {
	result := Result{}
	// Process the question
	d, err := svc.Retrieve(question.Text, limit)
	docs := ParseEvaluatedDocuments(d)
	if err != nil {
		return nil, err
	}

	svcImpl, ok := svc.(*rag.RagService)
	if !ok {
		return nil, fmt.Errorf("Invalid type of Rag. Cannot cast to RagService")
	}

	correctness := false
	relevantDocs := 0
	relevantVector := make([]bool, len(docs))
	rankedPrecisionVector := make([]float32, len(docs))
	rankedRecallVector := make([]float32, len(docs))

	if len(docs) != 0 {
		expectedSource := (*questionTable)[question.Text]
		correctness = expectedSource == docs[0].Source
		fmt.Printf("Para el overlap [%d]\n", svcImpl.GetOverlap())
		fmt.Printf("Pregunta [%s] \nValor en cache {%s}\nResultado esperado: %s\n", question.Text, expectedSource, docs[0].Source)
		result.MaxScore = float64(docs[0].Score)
		for i, doc := range docs {
			if expectedSource == doc.Source {
				relevantVector[i] = true
				relevantDocs++
			} else {
				relevantVector[i] = false
			}
			rankedPrecisionVector[i] = float32(relevantDocs) / float32(i+1)
		}
	}
	fmt.Printf("RELEVANT VECTOR %v\n", relevantVector)
	result.Correctness = correctness
	result.RelevantDocs = int64(relevantDocs)
	result.Documents = docs
	result.Overlap = int(svcImpl.GetOverlap())
	result.MeanScore = float64(GetMeanScore(docs))
	result.RankedPrecision = rankedPrecisionVector

	// This must be done here, because the recall needs the total relevant documents
	if len(docs) != 0 {
		counter := 0
		for i, relevant := range relevantVector {
			if relevant {
				counter++
			}
			if relevantDocs > 0 {
				rankedRecallVector[i] = float32(counter) / float32(relevantDocs)
			}

		}
	}
	result.RankedRecall = rankedRecallVector

	return &result, nil
}

func GetStepResult(question EvaluationQuestion, questionTable *map[string]string, svc rag.Rag, limit int) (*StepResult, error) {
	result := StepResult{}
	// Process the question
	d, err := svc.Retrieve(question.Text, limit)
	docs := ParseEvaluatedDocuments(d)
	if err != nil {
		return nil, err
	}

	svcImpl, ok := svc.(*rag.RagService)
	if !ok {
		return nil, fmt.Errorf("Invalid type of Rag. Cannot cast to RagService")
	}

	correctness := false
	relevantDocs := 0
	relevantVector := make([]bool, len(docs))
	rankedPrecisionVector := make([]float32, len(docs))
	rankedRecallVector := make([]float32, len(docs))

	if len(docs) != 0 {
		expectedSource := (*questionTable)[question.Text]
		correctness = expectedSource == docs[0].Source
		fmt.Printf("Para el overlap [%d]\n", svcImpl.GetOverlap())
		fmt.Printf("Pregunta [%s] \nValor en cache {%s}\nResultado esperado: %s\n", question.Text, expectedSource, docs[0].Source)
		result.MaxScore = float64(docs[0].Score)
		for i, doc := range docs {
			if expectedSource == doc.Source {
				relevantVector[i] = true
				relevantDocs++
			} else {
				relevantVector[i] = false
			}
			rankedPrecisionVector[i] = float32(relevantDocs) / float32(i+1)
		}
	}
	fmt.Printf("RELEVANT VECTOR %v\n", relevantVector)
	result.Correctness = correctness
	result.RelevantDocs = int64(relevantDocs)
	result.Documents = docs
	result.Step = int(svcImpl.GetStep())
	result.MeanScore = float64(GetMeanScore(docs))
	result.RankedPrecision = rankedPrecisionVector

	// This must be done here, because the recall needs the total relevant documents
	if len(docs) != 0 {
		counter := 0
		for i, relevant := range relevantVector {
			if relevant {
				counter++
			}
			if relevantDocs > 0 {
				rankedRecallVector[i] = float32(counter) / float32(relevantDocs)
			}

		}
	}
	result.RankedRecall = rankedRecallVector

	return &result, nil
}

func CreateOverlapEvaluation(overlaps []int64, step int64, vsize uint64, q []EvaluationQuestion, filepaths []string) Evaluation {
	return &EvaluationOverlapsImpl{
		Filepaths:  filepaths,
		Overlaps:   overlaps,
		Step:       step,
		VectorSize: vsize,
		questions:  q,
	}
}

func GetMeanScore(data []EvaluatedDocument) float32 {
	if len(data) == 0 {
		return 0.0
	}

	total := float32(len(data))
	acc := float32(0.0)

	for _, doc := range data {
		acc += doc.Score
	}

	return float32(acc / total)
}

// STEPS

type EvaluationStepsImpl struct {
	Steps      []int64
	Overlap    int64
	VectorSize uint64
	questions  []EvaluationQuestion
	Filepaths  []string
}

func (e *EvaluationStepsImpl) Load() error {
	const COLLECTION_NAME = "evaluation_step_size_{size}"
	const THREASHOLD = 0.655
	steps := e.Steps
	overlap := e.Overlap
	vsize := e.VectorSize
	services := make([]rag.Rag, 0)
	for i := range steps {
		step := steps[i]
		collection := strings.ReplaceAll(COLLECTION_NAME, "{size}", strconv.Itoa(int(step)))
		svc := rag.NewRagService(collection, vsize, THREASHOLD, step, overlap)
		services = append(services, svc)
	}

	for _, svc := range services {

		var wait sync.WaitGroup
		var readErr error
		ragservice := svc.(*rag.RagService)
		wait.Go(func() {
			for _, file := range e.Filepaths {

				fmt.Printf("Proccessing file %s for Size %d\n", file, ragservice.GetStep())
				if err := svc.ProcessFile(file); err != nil {
					readErr = err
				}
			}
		})

		if readErr != nil {
			return readErr
		}

		// Write in the database should be done
		wait.Wait()
	}
	return nil
}

// Example of evaluation:
//
// Any other evaluation struct can inherit from the Evaluation interfaace
// and implement its own evaluation. Depending on the used source the developer may
// have to program its own implementation.
//
// We have N services ready to process the evaluation algorithm
// Each service will:
// 1. Load the information into Qdrant
// 2. Use the retrieval pipeline -Ask a question
// 3. Track the score for the Retrieval
//
// Process will take the K closes documents for the input. Check LIMIT to change it's value
// Maybe it's a good idea to pass the value to the function in order to configure it.
func (e *EvaluationStepsImpl) Process(input EvaluationInput) ([]EvaluationOutput, error) {

	const COLLECTION_NAME = "evaluation_step_size_{size}"
	const THREASHOLD = 0.655
	const LIMIT = 10
	steps := e.Steps
	overlap := e.Overlap
	vsize := e.VectorSize
	services := make([]rag.Rag, 0)

	evals := make([]EvaluationOutput, 0)

	questionTable := make(map[string]string)

	stepInput, ok := (input).(*StepEvaluationInput)
	if !ok {
		// Always check 'ok' so your app doesn't panic if the type is wrong
		return make([]EvaluationOutput, 0),fmt.Errorf("expected *StepEvaluationInput, got %T", input)
	}

	for _, q := range stepInput.Questions {
		questionTable[q.Text] = q.ExpectedResult
	}

	for i := range steps {
		step := steps[i]
		collection := strings.ReplaceAll(COLLECTION_NAME, "{size}", strconv.Itoa(int(step)))
		svc := rag.NewRagService(collection, vsize, THREASHOLD, step, overlap)
		services = append(services, svc)
	}

	for _, question := range e.questions {

		stepEval := EvaluationStepOutput{
			Question: question.Text,
			Results:  make([]StepResult, 0),
		}

		for _, svc := range services {

			result, err := GetStepResult(question, &questionTable, svc, LIMIT)
			if err != nil {
				return []EvaluationOutput{}, err
			}
			if result == nil {
				panic("[Error] Result cannot be null")
			}
			stepEval.Results = append(stepEval.Results, *result)

		}

		evals = append(evals, stepEval)

	}

	return evals, nil

}


func CreateStepEvaluation(steps []int64, overlap int64, vsize uint64, q []EvaluationQuestion, filepaths []string) Evaluation {
	return &EvaluationStepsImpl{
		Filepaths:  filepaths,
		Steps:      steps,
		Overlap:    overlap,
		VectorSize: vsize,
		questions:  q,
	}
}
