"""Pytest configuration and shared fixtures."""

import os
import pytest
from unittest.mock import AsyncMock, MagicMock, patch

# Set test environment
os.environ["RAG_ENVIRONMENT"] = "development"
os.environ["RAG_DEBUG"] = "true"
os.environ["RAG_QDRANT_HOST"] = "memory"
os.environ["RAG_LOG_FORMAT"] = "console"


@pytest.fixture
def mock_openai_embeddings():
    """Mock OpenAI embeddings."""
    with patch("langchain_openai.OpenAIEmbeddings") as mock:
        instance = MagicMock()
        instance.aembed_documents = AsyncMock(
            return_value=[[0.1] * 1536 for _ in range(5)]
        )
        instance.aembed_query = AsyncMock(return_value=[0.1] * 1536)
        mock.return_value = instance
        yield mock


@pytest.fixture
def mock_openai_chat():
    """Mock OpenAI chat completion."""
    with patch("langchain_openai.ChatOpenAI") as mock:
        instance = MagicMock()
        response = MagicMock()
        response.content = "This is a test response."
        instance.ainvoke = AsyncMock(return_value=response)
        mock.return_value = instance
        yield mock


@pytest.fixture
def sample_documents():
    """Sample documents for testing."""
    return [
        {
            "content": "Python is a programming language known for its simplicity.",
            "metadata": {
                "filename": "python.md",
                "doc_id": "doc1",
                "chunk_index": 0,
            },
            "score": 0.95,
        },
        {
            "content": "FastAPI is a modern web framework for building APIs with Python.",
            "metadata": {
                "filename": "fastapi.md",
                "doc_id": "doc2",
                "chunk_index": 0,
            },
            "score": 0.90,
        },
        {
            "content": "LangChain is a framework for developing LLM applications.",
            "metadata": {
                "filename": "langchain.md",
                "doc_id": "doc3",
                "chunk_index": 0,
            },
            "score": 0.85,
        },
    ]


@pytest.fixture
def sample_text():
    """Sample text for testing."""
    return """
    This is a sample document about machine learning.

    Machine learning is a subset of artificial intelligence that enables
    systems to learn and improve from experience without being explicitly
    programmed. It focuses on developing algorithms that can access data
    and use it to learn for themselves.

    Key concepts in machine learning include:
    - Supervised learning
    - Unsupervised learning
    - Reinforcement learning

    Popular frameworks include TensorFlow, PyTorch, and scikit-learn.
    """


@pytest.fixture
def mock_vector_store(sample_documents):
    """Mock vector store."""
    with patch("src.rag.retrieval.vector_store.get_vector_store") as mock:
        instance = MagicMock()
        instance.search = AsyncMock(return_value=sample_documents)
        instance.add_documents = AsyncMock(return_value=["id1", "id2", "id3"])
        instance.health_check = AsyncMock(return_value=True)
        instance.initialize = AsyncMock()
        mock.return_value = instance
        yield instance


@pytest.fixture
async def async_client():
    """Create async test client for FastAPI."""
    from httpx import AsyncClient, ASGITransport
    from src.main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        yield client
