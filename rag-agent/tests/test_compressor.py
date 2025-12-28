"""Tests for context compression module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from langchain_core.documents import Document

from src.rag.retrieval.compressor import (
    ExtractiveSummaryCompressor,
    get_compressor,
)


class TestExtractiveSummaryCompressor:
    """Tests for ExtractiveSummaryCompressor class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        compressor = ExtractiveSummaryCompressor()
        assert compressor.compression_ratio == 0.3
        assert compressor.min_sentences == 2
        assert compressor.max_sentences == 10

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        compressor = ExtractiveSummaryCompressor(
            compression_ratio=0.5,
            min_sentences=1,
            max_sentences=5,
        )
        assert compressor.compression_ratio == 0.5
        assert compressor.min_sentences == 1
        assert compressor.max_sentences == 5

    def test_split_sentences(self):
        """Test sentence splitting."""
        compressor = ExtractiveSummaryCompressor()
        text = "First sentence. Second sentence! Third sentence?"
        sentences = compressor._split_sentences(text)
        assert len(sentences) == 3
        assert "First sentence" in sentences[0]

    def test_score_sentence_first_position(self):
        """Test that first sentence gets position bonus."""
        compressor = ExtractiveSummaryCompressor()
        score_first = compressor._score_sentence(
            "Important opening.", 0, 5, {"important"}
        )
        score_middle = compressor._score_sentence(
            "Important opening.", 2, 5, {"important"}
        )
        assert score_first > score_middle

    def test_score_sentence_query_overlap(self):
        """Test that query term overlap increases score."""
        compressor = ExtractiveSummaryCompressor()
        query_terms = {"python", "programming"}

        score_high = compressor._score_sentence(
            "Python is a programming language.", 1, 5, query_terms
        )
        score_low = compressor._score_sentence(
            "The weather is nice today.", 1, 5, query_terms
        )
        assert score_high > score_low

    @pytest.mark.asyncio
    async def test_compress_empty_list(self):
        """Test compressing empty document list."""
        compressor = ExtractiveSummaryCompressor()
        result = await compressor.compress("test query", [])
        assert result == []

    @pytest.mark.asyncio
    async def test_compress_short_document(self):
        """Test compressing document with few sentences."""
        compressor = ExtractiveSummaryCompressor(min_sentences=2)
        docs = [
            Document(page_content="Short sentence.", metadata={"id": 1})
        ]
        result = await compressor.compress("test", docs)
        assert len(result) == 1
        # Short document should be returned as-is
        assert result[0].page_content == docs[0].page_content

    @pytest.mark.asyncio
    async def test_compress_long_document(self):
        """Test compressing document with many sentences."""
        compressor = ExtractiveSummaryCompressor(
            compression_ratio=0.3,
            min_sentences=2,
            max_sentences=5,
        )
        content = ". ".join([f"This is sentence number {i}" for i in range(20)])
        docs = [Document(page_content=content, metadata={"id": 1})]

        result = await compressor.compress("sentence number", docs)

        assert len(result) == 1
        # Compressed content should be shorter
        assert len(result[0].page_content) < len(content)
        # Metadata should indicate compression
        assert result[0].metadata.get("compressed") is True


class TestGetCompressor:
    """Tests for get_compressor factory function."""

    def test_get_extractive_compressor(self):
        """Test getting extractive compressor."""
        compressor = get_compressor("extractive")
        assert isinstance(compressor, ExtractiveSummaryCompressor)

    def test_get_compressor_default(self):
        """Test default compressor is extractive."""
        compressor = get_compressor()
        assert isinstance(compressor, ExtractiveSummaryCompressor)

    def test_get_compressor_with_kwargs(self):
        """Test passing kwargs to compressor."""
        compressor = get_compressor(
            "extractive",
            compression_ratio=0.5,
            max_sentences=3,
        )
        assert compressor.compression_ratio == 0.5
        assert compressor.max_sentences == 3
