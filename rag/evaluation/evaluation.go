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
	Process(*OverlapEvaluationInput) ([]EvaluationOverlapOutput, error)
}

// EvaluationOverlapsImpl hold the parameters in order to evaluate different parameters for the RAG implementation
type EvaluationOverlapsImpl struct {
	Overlaps   []int64
	Step       int64
	VectorSize uint64
	questions  []EvaluationQuestion
	Filepaths  []string
}

type EvaluationStepsImpl struct {
	Overlap    int64
	Steps      int64
	VectorSize uint64
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
		wait.Go(func() {
			for _, file := range e.Filepaths {

				fmt.Printf("Proccessing file %s\n", file)
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

// We have N services ready to process the evaluation algorithm
// Each servic will:
// 1. Load the information into Qdrant
// 2. Use the retrieval pipeline -Ask a question
// 3. Track the score for the Retrieval
func (e *EvaluationOverlapsImpl) Process(input *OverlapEvaluationInput) ([]EvaluationOverlapOutput, error) {

	const COLLECTION_NAME = "evaluation_overlap_size_{size}"
	const THREASHOLD = 0.655
	overlaps := e.Overlaps
	step := e.Step
	vsize := e.VectorSize
	services := make([]rag.Rag, 0)


	evals := make([]EvaluationOverlapOutput, 0)

	questionTable := make(map[string]string)

	for _, q := range input.Questions {
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
			result := Result{}
			// Process the question
			d, err := svc.Retrieve(question.Text)
			docs := ParseEvaluatedDocuments(d)
			if err != nil {
				return []EvaluationOverlapOutput{}, err
			}

			svcImpl, ok := svc.(*rag.RagService)
			if !ok {
				return []EvaluationOverlapOutput{}, fmt.Errorf("Invalid type of Rag. Cannot cast to RagService")
			}
			
			correctness := false
			if len(docs) != 0 {
				correctness = questionTable[question.Text] == docs[0].Source
				fmt.Printf("Para el overlap [%d]\n", svcImpl.GetOverlap())
				fmt.Printf("Pregunta [%s] \nValor en cache {%s}\nResultado esperado: %s\n", question.Text, questionTable[question.Text], docs[0].Source)
				result.MaxScore = float64(docs[0].Score)
			}
			result.Correctness = correctness

			result.Documents = docs
			result.Overlap = int(svcImpl.GetOverlap())
			result.MeanScore = float64(GetMeanScore(docs))
			overlapEval.Results = append(overlapEval.Results, result)

		}
		
		evals = append(evals, overlapEval)

	}

	return evals, nil

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
