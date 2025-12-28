"""Tests for vector store module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from langchain_core.documents import Document

from src.rag.retrieval.vector_store import (
    VectorStore,
    get_vector_store,
    init_vector_store,
)


class TestVectorStore:
    """Tests for VectorStore class."""

    def test_init(self):
        """Test VectorStore initialization."""
        with patch("src.rag.retrieval.vector_store.get_settings") as mock_settings:
            mock_settings.return_value.qdrant_host = "localhost"
            mock_settings.return_value.qdrant_port = 6333
            mock_settings.return_value.qdrant_api_key = None

            store = VectorStore()

            assert store._client is None
            assert store._embeddings is None
            assert store.COLLECTION_NAME == "documents"
            assert store.VECTOR_SIZE == 1536

    @pytest.mark.asyncio
    async def test_initialize_in_memory(self):
        """Test initialization with in-memory storage."""
        with patch("src.rag.retrieval.vector_store.get_settings") as mock_settings, \
             patch("src.rag.retrieval.vector_store.QdrantClient") as mock_qdrant, \
             patch("src.rag.retrieval.vector_store.OpenAIEmbeddings") as mock_embeddings:

            mock_settings.return_value.qdrant_host = "memory"
            mock_settings.return_value.qdrant_port = 6333
            mock_settings.return_value.qdrant_api_key = None
            mock_settings.return_value.embedding_model = "text-embedding-3-small"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_client = MagicMock()
            mock_collections = MagicMock()
            mock_collections.collections = []
            mock_client.get_collections.return_value = mock_collections
            mock_qdrant.return_value = mock_client

            store = VectorStore()
            await store.initialize()

            # Should use in-memory client
            mock_qdrant.assert_called_with(":memory:")
            mock_client.create_collection.assert_called_once()

    @pytest.mark.asyncio
    async def test_initialize_existing_collection(self):
        """Test initialization when collection already exists."""
        with patch("src.rag.retrieval.vector_store.get_settings") as mock_settings, \
             patch("src.rag.retrieval.vector_store.QdrantClient") as mock_qdrant, \
             patch("src.rag.retrieval.vector_store.OpenAIEmbeddings"):

            mock_settings.return_value.qdrant_host = "memory"
            mock_settings.return_value.qdrant_port = 6333
            mock_settings.return_value.qdrant_api_key = None
            mock_settings.return_value.embedding_model = "text-embedding-3-small"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_collection = MagicMock()
            mock_collection.name = "documents"

            mock_collections = MagicMock()
            mock_collections.collections = [mock_collection]

            mock_client = MagicMock()
            mock_client.get_collections.return_value = mock_collections
            mock_qdrant.return_value = mock_client

            store = VectorStore()
            await store.initialize()

            # Should not create collection if it exists
            mock_client.create_collection.assert_not_called()

    @pytest.mark.asyncio
    async def test_add_documents(self):
        """Test adding documents to vector store."""
        with patch("src.rag.retrieval.vector_store.get_settings") as mock_settings:
            mock_settings.return_value.qdrant_host = "memory"
            mock_settings.return_value.qdrant_port = 6333

            store = VectorStore()

            # Mock client and embeddings
            mock_client = MagicMock()
            store._client = mock_client

            mock_embeddings = AsyncMock()
            mock_embeddings.aembed_documents.return_value = [
                [0.1] * 1536,
                [0.2] * 1536,
            ]
            store._embeddings = mock_embeddings

            docs = [
                Document(page_content="Content 1", metadata={"source": "test"}),
                Document(page_content="Content 2", metadata={"source": "test"}),
            ]

            chunk_ids = await store.add_documents(
                documents=docs,
                doc_id="doc123",
                metadata={"extra": "data"},
            )

            assert len(chunk_ids) == 2
            mock_client.upsert.assert_called_once()

            # Verify the points structure
            call_args = mock_client.upsert.call_args
            assert call_args.kwargs["collection_name"] == "documents"
            points = call_args.kwargs["points"]
            assert len(points) == 2

    @pytest.mark.asyncio
    async def test_add_documents_empty_list(self):
        """Test adding empty document list."""
        store = VectorStore()
        store._client = MagicMock()
        store._embeddings = AsyncMock()

        result = await store.add_documents([], "doc123")

        assert result == []
        store._client.upsert.assert_not_called()

    @pytest.mark.asyncio
    async def test_add_documents_not_initialized(self):
        """Test adding documents when not initialized."""
        store = VectorStore()

        with pytest.raises(RuntimeError, match="not initialized"):
            await store.add_documents(
                [Document(page_content="test")],
                "doc123",
            )

    @pytest.mark.asyncio
    async def test_search(self):
        """Test searching documents."""
        store = VectorStore()

        mock_client = MagicMock()
        mock_result_1 = MagicMock()
        mock_result_1.score = 0.9
        mock_result_1.payload = {
            "content": "Result 1 content",
            "doc_id": "doc1",
            "chunk_index": 0,
        }

        mock_result_2 = MagicMock()
        mock_result_2.score = 0.8
        mock_result_2.payload = {
            "content": "Result 2 content",
            "doc_id": "doc2",
            "chunk_index": 0,
        }

        mock_client.search.return_value = [mock_result_1, mock_result_2]
        store._client = mock_client

        mock_embeddings = AsyncMock()
        mock_embeddings.aembed_query.return_value = [0.1] * 1536
        store._embeddings = mock_embeddings

        results = await store.search("test query", top_k=5)

        assert len(results) == 2
        assert results[0]["content"] == "Result 1 content"
        assert results[0]["score"] == 0.9
        assert results[0]["metadata"]["doc_id"] == "doc1"
        assert results[1]["score"] == 0.8

    @pytest.mark.asyncio
    async def test_search_with_filter(self):
        """Test searching with metadata filter."""
        store = VectorStore()

        mock_client = MagicMock()
        mock_client.search.return_value = []
        store._client = mock_client

        mock_embeddings = AsyncMock()
        mock_embeddings.aembed_query.return_value = [0.1] * 1536
        store._embeddings = mock_embeddings

        await store.search(
            "test query",
            filter_metadata={"doc_id": "specific-doc"},
        )

        # Verify filter was passed
        call_args = mock_client.search.call_args
        assert call_args.kwargs["query_filter"] is not None

    @pytest.mark.asyncio
    async def test_search_not_initialized(self):
        """Test searching when not initialized."""
        store = VectorStore()

        with pytest.raises(RuntimeError, match="not initialized"):
            await store.search("test query")

    @pytest.mark.asyncio
    async def test_delete_document(self):
        """Test deleting a document."""
        store = VectorStore()

        mock_point_1 = MagicMock()
        mock_point_1.id = "chunk1"
        mock_point_2 = MagicMock()
        mock_point_2.id = "chunk2"

        mock_client = MagicMock()
        mock_client.scroll.return_value = ([mock_point_1, mock_point_2], None)
        store._client = mock_client

        deleted_count = await store.delete_document("doc123")

        assert deleted_count == 2
        mock_client.delete.assert_called_once()

    @pytest.mark.asyncio
    async def test_delete_document_not_found(self):
        """Test deleting non-existent document."""
        store = VectorStore()

        mock_client = MagicMock()
        mock_client.scroll.return_value = ([], None)
        store._client = mock_client

        deleted_count = await store.delete_document("nonexistent")

        assert deleted_count == 0
        mock_client.delete.assert_not_called()

    @pytest.mark.asyncio
    async def test_list_documents(self):
        """Test listing documents."""
        store = VectorStore()

        mock_point_1 = MagicMock()
        mock_point_1.payload = {
            "doc_id": "doc1",
            "filename": "file1.pdf",
            "content_type": "application/pdf",
        }

        mock_point_2 = MagicMock()
        mock_point_2.payload = {
            "doc_id": "doc1",  # Same doc, different chunk
            "filename": "file1.pdf",
            "content_type": "application/pdf",
        }

        mock_point_3 = MagicMock()
        mock_point_3.payload = {
            "doc_id": "doc2",
            "filename": "file2.txt",
            "content_type": "text/plain",
        }

        mock_client = MagicMock()
        mock_client.scroll.return_value = ([mock_point_1, mock_point_2, mock_point_3], None)
        store._client = mock_client

        docs = await store.list_documents()

        # Should have 2 unique documents
        assert len(docs) == 2
        doc1 = next(d for d in docs if d["id"] == "doc1")
        assert doc1["chunks_count"] == 2

    @pytest.mark.asyncio
    async def test_count_documents(self):
        """Test counting documents."""
        store = VectorStore()

        mock_point_1 = MagicMock()
        mock_point_1.payload = {"doc_id": "doc1"}
        mock_point_2 = MagicMock()
        mock_point_2.payload = {"doc_id": "doc1"}
        mock_point_3 = MagicMock()
        mock_point_3.payload = {"doc_id": "doc2"}

        mock_client = MagicMock()
        mock_client.scroll.return_value = ([mock_point_1, mock_point_2, mock_point_3], None)
        store._client = mock_client

        count = await store.count_documents()

        assert count == 2

    @pytest.mark.asyncio
    async def test_get_collection_info(self):
        """Test getting collection info."""
        store = VectorStore()

        mock_info = MagicMock()
        mock_info.vectors_count = 100
        mock_info.points_count = 50
        mock_info.status.value = "green"

        mock_client = MagicMock()
        mock_client.get_collection.return_value = mock_info
        store._client = mock_client

        info = await store.get_collection_info()

        assert info["name"] == "documents"
        assert info["vectors_count"] == 100
        assert info["points_count"] == 50
        assert info["status"] == "green"

    @pytest.mark.asyncio
    async def test_health_check_healthy(self):
        """Test health check when healthy."""
        store = VectorStore()

        mock_client = MagicMock()
        mock_client.get_collections.return_value = MagicMock()
        store._client = mock_client

        result = await store.health_check()

        assert result is True

    @pytest.mark.asyncio
    async def test_health_check_not_initialized(self):
        """Test health check when not initialized."""
        store = VectorStore()

        result = await store.health_check()

        assert result is False

    @pytest.mark.asyncio
    async def test_health_check_unhealthy(self):
        """Test health check when unhealthy."""
        store = VectorStore()

        mock_client = MagicMock()
        mock_client.get_collections.side_effect = Exception("Connection error")
        store._client = mock_client

        result = await store.health_check()

        assert result is False


class TestGetVectorStore:
    """Tests for get_vector_store function."""

    def test_get_vector_store_singleton(self):
        """Test that get_vector_store returns singleton."""
        with patch("src.rag.retrieval.vector_store._vector_store", None), \
             patch("src.rag.retrieval.vector_store.get_settings") as mock_settings:

            mock_settings.return_value.qdrant_host = "localhost"
            mock_settings.return_value.qdrant_port = 6333

            # Reset global
            import src.rag.retrieval.vector_store as vs_module
            vs_module._vector_store = None

            store1 = get_vector_store()
            store2 = get_vector_store()

            assert store1 is store2


class TestInitVectorStore:
    """Tests for init_vector_store function."""

    @pytest.mark.asyncio
    async def test_init_vector_store(self):
        """Test init_vector_store function."""
        with patch("src.rag.retrieval.vector_store.get_vector_store") as mock_get:
            mock_store = AsyncMock()
            mock_get.return_value = mock_store

            result = await init_vector_store()

            mock_store.initialize.assert_called_once()
            assert result is mock_store
