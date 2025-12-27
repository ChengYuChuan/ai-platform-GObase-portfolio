"""Vector store implementation using Qdrant."""

from typing import Any
from uuid import uuid4

from langchain_core.documents import Document
from langchain_openai import OpenAIEmbeddings
from qdrant_client import QdrantClient, models
from qdrant_client.http.exceptions import UnexpectedResponse

from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)

# Global vector store instance
_vector_store: "VectorStore | None" = None


class VectorStore:
    """Qdrant-based vector store for document embeddings."""

    COLLECTION_NAME = "documents"
    VECTOR_SIZE = 1536  # OpenAI text-embedding-3-small dimension

    def __init__(self) -> None:
        """Initialize the vector store."""
        self.settings = get_settings()
        self._client: QdrantClient | None = None
        self._embeddings: OpenAIEmbeddings | None = None

    async def initialize(self) -> None:
        """Initialize the Qdrant client and collection."""
        logger.info(
            "Initializing vector store",
            host=self.settings.qdrant_host,
            port=self.settings.qdrant_port,
        )

        # Initialize Qdrant client
        if self.settings.qdrant_host == "memory":
            # Use in-memory storage for development/testing
            self._client = QdrantClient(":memory:")
            logger.info("Using in-memory Qdrant storage")
        else:
            self._client = QdrantClient(
                host=self.settings.qdrant_host,
                port=self.settings.qdrant_port,
                api_key=self.settings.qdrant_api_key or None,
            )

        # Initialize embeddings
        self._embeddings = OpenAIEmbeddings(
            model=self.settings.embedding_model,
            openai_api_key=self.settings.openai_api_key,
        )

        # Create collection if it doesn't exist
        await self._ensure_collection()

        logger.info("Vector store initialized successfully")

    async def _ensure_collection(self) -> None:
        """Ensure the documents collection exists."""
        if self._client is None:
            raise RuntimeError("Vector store not initialized")

        try:
            collections = self._client.get_collections()
            collection_names = [c.name for c in collections.collections]

            if self.COLLECTION_NAME not in collection_names:
                self._client.create_collection(
                    collection_name=self.COLLECTION_NAME,
                    vectors_config=models.VectorParams(
                        size=self.VECTOR_SIZE,
                        distance=models.Distance.COSINE,
                    ),
                )
                logger.info(
                    "Created collection",
                    collection=self.COLLECTION_NAME,
                    vector_size=self.VECTOR_SIZE,
                )
            else:
                logger.info(
                    "Collection already exists",
                    collection=self.COLLECTION_NAME,
                )

        except Exception as e:
            logger.error("Failed to ensure collection", error=str(e))
            raise

    async def add_documents(
        self,
        documents: list[Document],
        doc_id: str,
        metadata: dict[str, Any] | None = None,
    ) -> list[str]:
        """Add documents to the vector store.

        Args:
            documents: List of LangChain documents to add.
            doc_id: Parent document ID.
            metadata: Additional metadata to attach.

        Returns:
            List of chunk IDs.
        """
        if self._client is None or self._embeddings is None:
            raise RuntimeError("Vector store not initialized")

        if not documents:
            return []

        # Generate embeddings for all documents
        texts = [doc.page_content for doc in documents]
        embeddings = await self._embeddings.aembed_documents(texts)

        # Prepare points for Qdrant
        points = []
        chunk_ids = []

        for i, (doc, embedding) in enumerate(zip(documents, embeddings)):
            chunk_id = str(uuid4())
            chunk_ids.append(chunk_id)

            # Combine document metadata with additional metadata
            point_metadata = {
                "doc_id": doc_id,
                "content": doc.page_content,
                "chunk_index": i,
                **doc.metadata,
            }
            if metadata:
                point_metadata.update(metadata)

            points.append(
                models.PointStruct(
                    id=chunk_id,
                    vector=embedding,
                    payload=point_metadata,
                )
            )

        # Upsert points to Qdrant
        self._client.upsert(
            collection_name=self.COLLECTION_NAME,
            points=points,
        )

        logger.info(
            "Added documents to vector store",
            doc_id=doc_id,
            num_chunks=len(points),
        )

        return chunk_ids

    async def search(
        self,
        query: str,
        top_k: int = 5,
        filter_metadata: dict[str, Any] | None = None,
        score_threshold: float | None = None,
    ) -> list[dict[str, Any]]:
        """Search for similar documents.

        Args:
            query: Search query text.
            top_k: Number of results to return.
            filter_metadata: Optional metadata filters.
            score_threshold: Minimum similarity score.

        Returns:
            List of search results with content, score, and metadata.
        """
        if self._client is None or self._embeddings is None:
            raise RuntimeError("Vector store not initialized")

        # Generate query embedding
        query_embedding = await self._embeddings.aembed_query(query)

        # Build filter if provided
        query_filter = None
        if filter_metadata:
            filter_conditions = [
                models.FieldCondition(
                    key=key,
                    match=models.MatchValue(value=value),
                )
                for key, value in filter_metadata.items()
            ]
            query_filter = models.Filter(must=filter_conditions)

        # Perform search
        results = self._client.search(
            collection_name=self.COLLECTION_NAME,
            query_vector=query_embedding,
            limit=top_k,
            query_filter=query_filter,
            score_threshold=score_threshold,
        )

        # Format results
        formatted_results = []
        for result in results:
            payload = result.payload or {}
            formatted_results.append(
                {
                    "content": payload.get("content", ""),
                    "score": result.score,
                    "metadata": {
                        k: v for k, v in payload.items() if k != "content"
                    },
                }
            )

        logger.debug(
            "Search completed",
            query_preview=query[:50],
            num_results=len(formatted_results),
        )

        return formatted_results

    async def delete_document(self, doc_id: str) -> int:
        """Delete all chunks for a document.

        Args:
            doc_id: Document ID to delete.

        Returns:
            Number of chunks deleted.
        """
        if self._client is None:
            raise RuntimeError("Vector store not initialized")

        # Find all points with this doc_id
        scroll_result = self._client.scroll(
            collection_name=self.COLLECTION_NAME,
            scroll_filter=models.Filter(
                must=[
                    models.FieldCondition(
                        key="doc_id",
                        match=models.MatchValue(value=doc_id),
                    )
                ]
            ),
            limit=1000,
            with_payload=False,
            with_vectors=False,
        )

        point_ids = [str(point.id) for point in scroll_result[0]]

        if point_ids:
            self._client.delete(
                collection_name=self.COLLECTION_NAME,
                points_selector=models.PointIdsList(points=point_ids),
            )

        logger.info(
            "Deleted document chunks",
            doc_id=doc_id,
            num_deleted=len(point_ids),
        )

        return len(point_ids)

    async def list_documents(
        self,
        limit: int = 20,
        offset: int = 0,
    ) -> list[dict[str, Any]]:
        """List all documents (grouped by doc_id).

        Args:
            limit: Maximum number of documents to return.
            offset: Number of documents to skip.

        Returns:
            List of document metadata.
        """
        if self._client is None:
            raise RuntimeError("Vector store not initialized")

        # Get all unique doc_ids
        # Note: This is a simplified implementation
        # For production, consider using a separate metadata store
        scroll_result = self._client.scroll(
            collection_name=self.COLLECTION_NAME,
            limit=1000,
            with_payload=True,
            with_vectors=False,
        )

        # Group by doc_id
        documents: dict[str, dict[str, Any]] = {}
        for point in scroll_result[0]:
            payload = point.payload or {}
            doc_id = payload.get("doc_id")
            if doc_id and doc_id not in documents:
                documents[doc_id] = {
                    "id": doc_id,
                    "filename": payload.get("filename", "unknown"),
                    "content_type": payload.get("content_type", "unknown"),
                    "chunks_count": 0,
                    "status": "processed",
                }
            if doc_id:
                documents[doc_id]["chunks_count"] += 1

        # Apply pagination
        doc_list = list(documents.values())
        return doc_list[offset : offset + limit]

    async def count_documents(self) -> int:
        """Count total number of unique documents.

        Returns:
            Number of unique documents.
        """
        if self._client is None:
            raise RuntimeError("Vector store not initialized")

        try:
            scroll_result = self._client.scroll(
                collection_name=self.COLLECTION_NAME,
                limit=10000,
                with_payload=["doc_id"],
                with_vectors=False,
            )

            unique_docs = set()
            for point in scroll_result[0]:
                payload = point.payload or {}
                doc_id = payload.get("doc_id")
                if doc_id:
                    unique_docs.add(doc_id)

            return len(unique_docs)

        except UnexpectedResponse:
            return 0

    async def get_collection_info(self) -> dict[str, Any]:
        """Get collection statistics.

        Returns:
            Collection information.
        """
        if self._client is None:
            raise RuntimeError("Vector store not initialized")

        try:
            info = self._client.get_collection(self.COLLECTION_NAME)
            return {
                "name": self.COLLECTION_NAME,
                "vectors_count": info.vectors_count,
                "points_count": info.points_count,
                "status": info.status.value,
            }
        except Exception as e:
            logger.error("Failed to get collection info", error=str(e))
            return {"error": str(e)}

    async def health_check(self) -> bool:
        """Check if the vector store is healthy.

        Returns:
            True if healthy, False otherwise.
        """
        if self._client is None:
            return False

        try:
            self._client.get_collections()
            return True
        except Exception:
            return False


def get_vector_store() -> VectorStore:
    """Get the global vector store instance.

    Returns:
        The vector store singleton.
    """
    global _vector_store
    if _vector_store is None:
        _vector_store = VectorStore()
    return _vector_store


async def init_vector_store() -> VectorStore:
    """Initialize and return the vector store.

    Returns:
        The initialized vector store.
    """
    store = get_vector_store()
    await store.initialize()
    return store
