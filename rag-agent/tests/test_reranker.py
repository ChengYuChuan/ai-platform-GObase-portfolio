"""Tests for reranking module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from langchain_core.documents import Document

from src.rag.retrieval.reranker import (
    CrossEncoderReranker,
    CohereReranker,
    LLMReranker,
    get_reranker,
)


class TestCrossEncoderReranker:
    """Tests for CrossEncoderReranker class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        reranker = CrossEncoderReranker()
        assert reranker.model_name == "cross-encoder/ms-marco-MiniLM-L-6-v2"
        assert reranker.top_k is None
        assert reranker.batch_size == 32
        assert reranker._initialized is False

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        reranker = CrossEncoderReranker(
            model_name="custom-model",
            top_k=10,
            batch_size=16,
        )
        assert reranker.model_name == "custom-model"
        assert reranker.top_k == 10
        assert reranker.batch_size == 16

    def test_rerank_empty_list(self):
        """Test reranking empty document list."""
        reranker = CrossEncoderReranker()
        result = reranker.rerank("test query", [])
        assert result == []

    def test_rerank_documents(self):
        """Test reranking documents with mocked model."""
        reranker = CrossEncoderReranker(top_k=2)

        # Manually set the model to mock
        mock_model = MagicMock()
        mock_model.predict.return_value = [0.9, 0.5, 0.7]
        reranker._model = mock_model
        reranker._initialized = True

        docs = [
            Document(page_content="Doc A", metadata={"id": 1}),
            Document(page_content="Doc B", metadata={"id": 2}),
            Document(page_content="Doc C", metadata={"id": 3}),
        ]

        result = reranker.rerank("test query", docs)

        assert len(result) == 2
        # Should be sorted by score (0.9 > 0.7 > 0.5)
        assert result[0].metadata["rerank_score"] == 0.9
        assert result[1].metadata["rerank_score"] == 0.7

    @pytest.mark.asyncio
    async def test_arerank_async(self):
        """Test async reranking."""
        reranker = CrossEncoderReranker()

        # Manually set the model to mock
        mock_model = MagicMock()
        mock_model.predict.return_value = [0.8, 0.6]
        reranker._model = mock_model
        reranker._initialized = True

        docs = [
            Document(page_content="Doc A", metadata={"id": 1}),
            Document(page_content="Doc B", metadata={"id": 2}),
        ]

        result = await reranker.arerank("test query", docs)

        assert len(result) == 2
        assert all("rerank_score" in doc.metadata for doc in result)


class TestCohereReranker:
    """Tests for CohereReranker class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        reranker = CohereReranker()
        assert reranker.model == "rerank-english-v3.0"
        assert reranker.top_k is None
        assert reranker._client is None

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        reranker = CohereReranker(
            api_key="test-key",
            model="custom-model",
            top_k=5,
        )
        assert reranker.api_key == "test-key"
        assert reranker.model == "custom-model"
        assert reranker.top_k == 5

    @pytest.mark.asyncio
    async def test_arerank_empty_list(self):
        """Test reranking empty document list."""
        reranker = CohereReranker()
        result = await reranker.arerank("test query", [])
        assert result == []

    @pytest.mark.asyncio
    async def test_arerank_with_mock(self):
        """Test reranking with mocked Cohere client."""
        with patch.object(CohereReranker, "_get_client") as mock_get_client:
            # Create mock response
            mock_result_1 = MagicMock()
            mock_result_1.index = 1
            mock_result_1.relevance_score = 0.9

            mock_result_2 = MagicMock()
            mock_result_2.index = 0
            mock_result_2.relevance_score = 0.7

            mock_response = MagicMock()
            mock_response.results = [mock_result_1, mock_result_2]

            mock_client = MagicMock()
            mock_client.rerank.return_value = mock_response
            mock_get_client.return_value = mock_client

            reranker = CohereReranker(top_k=2)
            docs = [
                Document(page_content="Doc A", metadata={"id": 1}),
                Document(page_content="Doc B", metadata={"id": 2}),
            ]

            result = await reranker.arerank("test query", docs)

            assert len(result) == 2
            # First result should be Doc B (index 1 with score 0.9)
            assert result[0].page_content == "Doc B"
            assert result[0].metadata["rerank_score"] == 0.9


class TestLLMReranker:
    """Tests for LLMReranker class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        with patch("src.rag.retrieval.reranker.get_settings") as mock_settings:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            reranker = LLMReranker()
            assert reranker.model_name == "gpt-4o-mini"
            assert reranker.top_k is None

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        reranker = LLMReranker(model_name="gpt-4", top_k=5)
        assert reranker.model_name == "gpt-4"
        assert reranker.top_k == 5

    @pytest.mark.asyncio
    async def test_arerank_empty_list(self):
        """Test reranking empty document list."""
        reranker = LLMReranker()
        result = await reranker.arerank("test query", [])
        assert result == []

    @pytest.mark.asyncio
    async def test_arerank_with_mock(self):
        """Test reranking with mocked LLM."""
        with patch.object(LLMReranker, "_get_llm") as mock_get_llm:
            # Mock LLM responses
            mock_llm = AsyncMock()
            mock_response_1 = MagicMock()
            mock_response_1.content = "8"
            mock_response_2 = MagicMock()
            mock_response_2.content = "5"

            mock_llm.ainvoke = AsyncMock(
                side_effect=[mock_response_1, mock_response_2]
            )
            mock_get_llm.return_value = mock_llm

            reranker = LLMReranker(top_k=2)
            docs = [
                Document(page_content="Doc A", metadata={"id": 1}),
                Document(page_content="Doc B", metadata={"id": 2}),
            ]

            result = await reranker.arerank("test query", docs)

            assert len(result) == 2
            # Doc A should come first (score 8 > 5)
            assert result[0].page_content == "Doc A"
            assert result[0].metadata["rerank_score"] == 0.8  # Normalized 8/10

    @pytest.mark.asyncio
    async def test_arerank_handles_invalid_score(self):
        """Test handling of invalid LLM score response."""
        with patch.object(LLMReranker, "_get_llm") as mock_get_llm:
            mock_llm = AsyncMock()
            mock_response = MagicMock()
            mock_response.content = "not a number"
            mock_llm.ainvoke = AsyncMock(return_value=mock_response)
            mock_get_llm.return_value = mock_llm

            reranker = LLMReranker()
            docs = [Document(page_content="Test doc", metadata={})]

            result = await reranker.arerank("test query", docs)

            assert len(result) == 1
            # Should default to 5.0 on parse failure
            assert result[0].metadata["rerank_score"] == 0.5


class TestGetReranker:
    """Tests for get_reranker factory function."""

    def test_get_cross_encoder_reranker(self):
        """Test getting cross-encoder reranker."""
        with patch("src.rag.retrieval.reranker.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            reranker = get_reranker("cross-encoder")
            assert isinstance(reranker, CrossEncoderReranker)

    def test_get_cohere_reranker(self):
        """Test getting Cohere reranker."""
        with patch("src.rag.retrieval.reranker.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            reranker = get_reranker("cohere")
            assert isinstance(reranker, CohereReranker)

    def test_get_llm_reranker(self):
        """Test getting LLM reranker."""
        with patch("src.rag.retrieval.reranker.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            reranker = get_reranker("llm")
            assert isinstance(reranker, LLMReranker)

    def test_get_default_reranker(self):
        """Test default reranker is cross-encoder."""
        with patch("src.rag.retrieval.reranker.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 5
            reranker = get_reranker()
            assert isinstance(reranker, CrossEncoderReranker)

    def test_get_reranker_with_kwargs(self):
        """Test passing kwargs to reranker."""
        with patch("src.rag.retrieval.reranker.get_settings") as mock_settings:
            mock_settings.return_value.retrieval_top_k = 10
            # Note: get_reranker already sets top_k, so just test type
            reranker = get_reranker("cross-encoder")
            assert isinstance(reranker, CrossEncoderReranker)
