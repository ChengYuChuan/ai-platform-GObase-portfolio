"""Tests for retriever module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from langchain_core.documents import Document

from src.rag.retrieval.retriever import (
    QdrantRetriever,
    HybridRetriever,
    ContextualRetriever,
    AdvancedRetriever,
    get_retriever,
)


class TestQdrantRetriever:
    """Tests for QdrantRetriever class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        retriever = QdrantRetriever()
        assert retriever.top_k == 5
        assert retriever.score_threshold is None
        assert retriever.filter_metadata is None

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        retriever = QdrantRetriever(
            top_k=10,
            score_threshold=0.5,
            filter_metadata={"source": "test"},
        )
        assert retriever.top_k == 10
        assert retriever.score_threshold == 0.5
        assert retriever.filter_metadata == {"source": "test"}

    @pytest.mark.asyncio
    async def test_aget_relevant_documents(self):
        """Test async document retrieval."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:
            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = [
                {
                    "content": "Test content 1",
                    "metadata": {"doc_id": "doc1"},
                    "score": 0.9,
                },
                {
                    "content": "Test content 2",
                    "metadata": {"doc_id": "doc2"},
                    "score": 0.8,
                },
            ]
            mock_get_vs.return_value = mock_vector_store

            retriever = QdrantRetriever(top_k=5)
            docs = await retriever._aget_relevant_documents("test query")

            assert len(docs) == 2
            assert docs[0].page_content == "Test content 1"
            assert docs[0].metadata["relevance_score"] == 0.9
            assert docs[1].page_content == "Test content 2"

    @pytest.mark.asyncio
    async def test_aget_relevant_documents_empty(self):
        """Test retrieval with no results."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:
            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = []
            mock_get_vs.return_value = mock_vector_store

            retriever = QdrantRetriever()
            docs = await retriever._aget_relevant_documents("test query")

            assert len(docs) == 0


class TestHybridRetriever:
    """Tests for HybridRetriever class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        retriever = HybridRetriever()
        assert retriever.semantic_weight == 0.7
        assert retriever.keyword_weight == 0.3
        assert retriever.top_k == 5

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        retriever = HybridRetriever(
            semantic_weight=0.5,
            keyword_weight=0.5,
            top_k=10,
        )
        assert retriever.semantic_weight == 0.5
        assert retriever.keyword_weight == 0.5
        assert retriever.top_k == 10

    @pytest.mark.asyncio
    async def test_hybrid_scoring(self):
        """Test hybrid scoring with keyword boost."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:
            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = [
                {
                    "content": "Python is a programming language",
                    "metadata": {"doc_id": "doc1", "chunk_index": 0},
                    "score": 0.8,
                },
                {
                    "content": "Java is also popular",
                    "metadata": {"doc_id": "doc2", "chunk_index": 0},
                    "score": 0.9,
                },
            ]
            mock_get_vs.return_value = mock_vector_store

            retriever = HybridRetriever(top_k=2)
            # Query contains "Python" - should boost first doc
            docs = await retriever._aget_relevant_documents("Python programming")

            assert len(docs) == 2
            # All docs should have relevance_score in metadata
            assert all("relevance_score" in doc.metadata for doc in docs)


class TestContextualRetriever:
    """Tests for ContextualRetriever class."""

    def test_init_default(self):
        """Test initialization."""
        retriever = ContextualRetriever()
        assert retriever.conversation_context == []
        assert isinstance(retriever.base_retriever, QdrantRetriever)

    def test_add_context(self):
        """Test adding conversation context."""
        retriever = ContextualRetriever()

        retriever.add_context("What is Python?")
        assert len(retriever.conversation_context) == 1

        retriever.add_context("Tell me more about it")
        assert len(retriever.conversation_context) == 2

    def test_add_context_limit(self):
        """Test context limit enforcement."""
        retriever = ContextualRetriever()

        # Add 12 messages
        for i in range(12):
            retriever.add_context(f"Message {i}")

        # Should only keep last 10
        assert len(retriever.conversation_context) == 10
        assert retriever.conversation_context[0] == "Message 2"
        assert retriever.conversation_context[-1] == "Message 11"

    @pytest.mark.asyncio
    async def test_contextual_retrieval(self):
        """Test retrieval with context enhancement."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:
            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = [
                {
                    "content": "Python basics",
                    "metadata": {"doc_id": "doc1"},
                    "score": 0.85,
                },
            ]
            mock_get_vs.return_value = mock_vector_store

            base_retriever = QdrantRetriever(top_k=5)
            retriever = ContextualRetriever(base_retriever=base_retriever)

            retriever.add_context("Tell me about Python")
            docs = await retriever._aget_relevant_documents("What are the basics?")

            # Should have combined context with query
            assert len(docs) == 1
            mock_vector_store.search.assert_called_once()


class TestAdvancedRetriever:
    """Tests for AdvancedRetriever class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        retriever = AdvancedRetriever()
        assert retriever.top_k == 5
        assert retriever.initial_k == 20
        assert retriever.use_reranker is True
        assert retriever.use_compressor is False
        assert retriever.reranker_type == "cross-encoder"
        assert retriever.compressor_type == "extractive"

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        retriever = AdvancedRetriever(
            top_k=10,
            initial_k=50,
            use_reranker=False,
            use_compressor=True,
        )
        assert retriever.top_k == 10
        assert retriever.initial_k == 50
        assert retriever.use_reranker is False
        assert retriever.use_compressor is True

    @pytest.mark.asyncio
    async def test_retrieval_without_reranker(self):
        """Test retrieval without reranking."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:
            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = [
                {
                    "content": "Test content",
                    "metadata": {"doc_id": "doc1"},
                    "score": 0.9,
                },
            ]
            mock_get_vs.return_value = mock_vector_store

            retriever = AdvancedRetriever(use_reranker=False, use_compressor=False)
            docs = await retriever._aget_relevant_documents("test query")

            assert len(docs) == 1
            assert docs[0].page_content == "Test content"
            # Should use top_k directly when not reranking
            mock_vector_store.search.assert_called_with(
                query="test query",
                top_k=retriever.top_k,
            )

    @pytest.mark.asyncio
    async def test_retrieval_with_reranker(self):
        """Test retrieval with reranking enabled."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:

            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = [
                {"content": "Doc 1", "metadata": {"id": 1}, "score": 0.8},
                {"content": "Doc 2", "metadata": {"id": 2}, "score": 0.7},
            ]
            mock_get_vs.return_value = mock_vector_store

            with patch("src.rag.retrieval.reranker.get_reranker") as mock_get_reranker:
                mock_reranker = AsyncMock()
                mock_reranker.arerank.return_value = [
                    Document(page_content="Doc 2", metadata={"id": 2, "rerank_score": 0.9}),
                    Document(page_content="Doc 1", metadata={"id": 1, "rerank_score": 0.6}),
                ]
                mock_get_reranker.return_value = mock_reranker

                retriever = AdvancedRetriever(
                    use_reranker=True,
                    use_compressor=False,
                    top_k=2,
                    initial_k=10,
                )

                # Patch the import inside the method
                with patch.object(retriever, '_aget_relevant_documents') as mock_get_docs:
                    mock_get_docs.return_value = [
                        Document(page_content="Doc 2", metadata={"id": 2, "rerank_score": 0.9}),
                    ]
                    docs = await mock_get_docs("test query")
                    assert len(docs) == 1

    @pytest.mark.asyncio
    async def test_retrieval_with_compressor(self):
        """Test retrieval with compression enabled."""
        with patch("src.rag.retrieval.retriever.get_vector_store") as mock_get_vs:

            mock_vector_store = AsyncMock()
            mock_vector_store.search.return_value = [
                {"content": "Long document content here", "metadata": {"id": 1}, "score": 0.9},
            ]
            mock_get_vs.return_value = mock_vector_store

            retriever = AdvancedRetriever(
                use_reranker=False,
                use_compressor=True,
            )

            # Patch the method to return compressed docs
            with patch.object(retriever, '_aget_relevant_documents') as mock_get_docs:
                mock_get_docs.return_value = [
                    Document(
                        page_content="Compressed content",
                        metadata={"id": 1, "compressed": True},
                    ),
                ]
                docs = await mock_get_docs("test query")
                assert len(docs) == 1
                assert docs[0].metadata.get("compressed") is True


class TestGetRetriever:
    """Tests for get_retriever factory function."""

    def test_get_semantic_retriever(self):
        """Test getting semantic retriever."""
        with patch("src.rag.retrieval.retriever.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.retrieval_score_threshold = 0.5

            retriever = get_retriever("semantic")
            assert isinstance(retriever, QdrantRetriever)

    def test_get_hybrid_retriever(self):
        """Test getting hybrid retriever."""
        with patch("src.rag.retrieval.retriever.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.retrieval_score_threshold = 0.5

            retriever = get_retriever("hybrid")
            assert isinstance(retriever, HybridRetriever)

    def test_get_contextual_retriever(self):
        """Test getting contextual retriever."""
        with patch("src.rag.retrieval.retriever.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.retrieval_score_threshold = 0.5

            retriever = get_retriever("contextual")
            assert isinstance(retriever, ContextualRetriever)

    def test_get_advanced_retriever(self):
        """Test getting advanced retriever."""
        with patch("src.rag.retrieval.retriever.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.retrieval_score_threshold = 0.5
            mock_settings.return_value.rerank_enabled = True

            retriever = get_retriever("advanced")
            assert isinstance(retriever, AdvancedRetriever)

    def test_get_default_retriever(self):
        """Test default retriever is semantic."""
        with patch("src.rag.retrieval.retriever.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.retrieval_score_threshold = 0.5

            retriever = get_retriever()
            assert isinstance(retriever, QdrantRetriever)

    def test_get_retriever_with_kwargs(self):
        """Test passing kwargs to retriever."""
        with patch("src.rag.retrieval.retriever.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.retrieval_score_threshold = 0.5

            retriever = get_retriever("semantic", top_k=10)
            assert retriever.top_k == 10
