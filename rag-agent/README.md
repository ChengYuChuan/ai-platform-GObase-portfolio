# RAG Agent Service

> 🚧 **Coming Soon** - Phase 2 of the portfolio

## Overview

A Python-based RAG (Retrieval-Augmented Generation) pipeline with autonomous agents.

## Planned Features

- **Document Processing**: PDF, DOCX, Markdown ingestion
- **Vector Search**: Hybrid search with Qdrant
- **RAG Pipeline**: LangChain-based retrieval and generation
- **Agent Framework**: LangGraph-based autonomous agents
- **Tool Integration**: Function calling with external APIs

## Tech Stack

| Component | Technology |
|-----------|------------|
| Framework | FastAPI |
| RAG | LangChain, LlamaIndex |
| Vector DB | Qdrant |
| Agents | LangGraph |
| Package Manager | uv |

## Why Python?

Python provides direct access to the AI/ML ecosystem:
- LangChain, LlamaIndex for RAG pipelines
- Native vector database clients
- Rich document processing libraries
- Mature agent frameworks

## Directory Structure (Planned)

```
rag-agent/
├── src/
│   ├── api/           # FastAPI routes
│   ├── rag/           # Retrieval pipeline
│   │   ├── ingestion/ # Document loaders
│   │   ├── retrieval/ # Vector search
│   │   └── pipeline.py
│   └── agents/        # LangGraph agents
├── tests/
├── pyproject.toml
└── Dockerfile
```

## Development Timeline

This project is planned for development after completing the Go Gateway (Phase 1).
