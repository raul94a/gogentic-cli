package evaluation

import rag "gogentic-rag"

type EvaluationOutput interface{
	Duck()
}

type EvaluationOverlapOutput struct {
	Question string   `json:"question"`
	Results  []Result `json:"results"`
}

func (e EvaluationOverlapOutput) Duck() {}

type Result struct {
	Correctness 	bool                `json:"correctness"`
	MaxScore    	float64             `json:"maxScore"`
	MeanScore   	float64             `json:"meanScore"`
	Overlap     	int                 `json:"overlap"`
	Documents   	[]EvaluatedDocument `json:"documents"`
	RankedPrecision []float32			`json:"rankedPrecision"`
	RankedRecall	[]float32			`json:"rankedRecall"`
	// number of documents relevant for the query
	RelevantDocs	int64				`json:"relevantDocs"`
}

// EvaluatedDocument is a result of the Retrieve from Rag for the evaluation process.
type EvaluatedDocument struct {
	ID         string  `json:"id"`
	Score      float32 `json:"score"`
	Source     string  `json:"source"`
	ChunkIndex int     `json:"chunk_index"`
}

func ParseEvaluatedDocuments(docs []rag.RetrievedDocument) []EvaluatedDocument {
	evs := make([]EvaluatedDocument, 0)
	for _, doc := range docs {
		ev := EvaluatedDocument{
			ID:         doc.ID,
			Score:      doc.Score,
			Source:     doc.Source,
			ChunkIndex: doc.ChunkIndex,
		}
		evs = append(evs, ev)
	}
	return evs
}


type EvaluationStepOutput struct {
	Question string   `json:"question"`
	Results  []StepResult `json:"results"`
}

func (e EvaluationStepOutput) Duck() {}

type StepResult struct {
	Correctness 	bool                `json:"correctness"`
	MaxScore    	float64             `json:"maxScore"`
	MeanScore   	float64             `json:"meanScore"`
	Step	    	int                 `json:"step"`
	Documents   	[]EvaluatedDocument `json:"documents"`
	RankedPrecision []float32			`json:"rankedPrecision"`
	RankedRecall	[]float32			`json:"rankedRecall"`
	// number of documents relevant for the query
	RelevantDocs	int64				`json:"relevantDocs"`
}