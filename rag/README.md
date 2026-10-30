## RAG


## Evaluation requirements

- Python 3.11+ installed. The matplotlib library should be installed system-widely or inside [the evaluation python folder](./rag/evaluation/python) as part of a virtual environment.

- Qdrant Database. Using a Docker container is recommended.

- Data loaded. If you need to run an evaluation of your retrieval system, you mostly need to load the data into you vector database. This can be done outside this library. However, we provide [The evaluation interface](rag\evaluation\evaluation.go) with [This implementation example](rag\evaluation\evaluation_test.go). The test will load you data from your referenced file/s by using the ENV variable RAG_TEST_FILEPATHS

```go
filepaths := os.Getenv("RAG_TEST_FILEPATHS")
```
*  The evaluation process can be performed by using one or several files. The provided implementation uses several files to chunk the information into the database. You will need to adapt you evaluation checks in order to properly measure the retrieval. For example, when developing this library, we tested against one book split up in one file per chapter. Of course, to chunk the entire book into the database was another solution. However, it would be more difficult to check if the retrieved document was correct. Because the source will be the same for all the documents -at least, in principle- we had to check a range of chunk index for automatically evaluating the query.

- Ollama + embedding model. The model used when developing this library was embeddinggemma-2:270m. The developer can use any of the available solutions to generate the vectors.