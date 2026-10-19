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


// EvaluatedDocument is a result of the Retrieve from Rag for the evaluation process.
type EvaluatedDocument struct {
	ID         string  `json:"id"`
	Score      float32 `json:"score"`
	Source     string  `json:"source"`
	ChunkIndex int     `json:"chunk_index"`
}

func ParseEvaluatedDocuments(docs []rag.RetrievedDocument) ([]EvaluatedDocument) {
	evs := make([]EvaluatedDocument, 0)
	for _, doc := range docs {
		ev := EvaluatedDocument {
			ID: doc.ID,
			Score: doc.Score,
			Source: doc.Source,
			ChunkIndex: doc.ChunkIndex,
		}
		evs = append(evs, ev)
	}
	return evs
}

type OverlapSize int64

type Analysis struct {
	Question string                                  `json:"question"`
	Results  map[OverlapSize][]EvaluatedDocument    `json:"results"`
}

type Evaluation interface {
	Load() error
	Process() ([]Analysis, error)
}

// EvaluationOverlapsImpl hold the parameters in order to evaluate different parameters for the RAG implementation
type EvaluationOverlapsImpl struct {
	Overlaps   []int64
	Step       int64
	VectorSize uint64
	questions  []string
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
func (e *EvaluationOverlapsImpl) Process() ([]Analysis, error) {

	const COLLECTION_NAME = "evaluation_overlap_size_{size}"
	const THREASHOLD = 0.655
	overlaps := e.Overlaps
	step := e.Step
	vsize := e.VectorSize
	services := make([]rag.Rag, 0)

	analysisResult := make([]Analysis, 0)

	for i := range overlaps {
		overlap := overlaps[i]

		collection := strings.ReplaceAll(COLLECTION_NAME, "{size}", strconv.Itoa(int(overlap)))
		svc := rag.NewRagService(collection, vsize, THREASHOLD, step, overlap)
		services = append(services, svc)
	}

	for _, question := range e.questions {
		analysis := Analysis{
			Question: question,
			Results:  make(map[OverlapSize][]EvaluatedDocument),
		}
		for _, svc := range services {
			// Process the question
			d, err := svc.Retrieve(question)
			docs := ParseEvaluatedDocuments(d)
			if err != nil {
				return []Analysis{}, err
			}

			svcImpl, ok := svc.(*rag.RagService)
			if !ok {
				return []Analysis{}, fmt.Errorf("Invalid type of Rag. Cannot cast to RagService")
			}
			analysis.Results[OverlapSize(svcImpl.GetOverlap())] = docs
		}
		analysisResult = append(analysisResult, analysis)

	}

	return analysisResult, nil

}

func CreateOverlapEvaluation(overlaps []int64, step int64, vsize uint64, q []string, filepaths []string) Evaluation {
	return &EvaluationOverlapsImpl{
		Filepaths:  filepaths,
		Overlaps:   overlaps,
		Step:       step,
		VectorSize: vsize,
		questions:  q,
	}
}
