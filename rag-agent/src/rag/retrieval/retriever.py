"""RAG retriever implementation using LangChain."""

from typing import Any

from langchain_core.documents import Document
from langchain_core.retrievers import BaseRetriever
from langchain_core.callbacks import CallbackManagerForRetrieverRun
from pydantic import Field

from src.core import get_logger
from src.core.config import get_settings
from src.rag.retrieval.vector_store import get_vector_store

logger = get_logger(__name__)


class QdrantRetriever(BaseRetriever):
    """Custom retriever using Qdrant vector store."""

    top_k: int = Field(default=5, description="Number of documents to retrieve")
    score_threshold: float | None = Field(default=None, description="Minimum similarity score")
    filter_metadata: dict[str, Any] | None = Field(default=None, description="Metadata filters")

    class Config:
        arbitrary_types_allowed = True

    def _get_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Synchronous retrieval - wraps async method."""
        import asyncio

        loop = asyncio.get_event_loop()
        return loop.run_until_complete(
            self._aget_relevant_documents(query, run_manager=run_manager)
        )

    async def _aget_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Retrieve relevant documents for a query.

        Args:
            query: Search query.
            run_manager: Optional callback manager.

        Returns:
            List of relevant documents.
        """
        vector_store = get_vector_store()

        results = await vector_store.search(
            query=query,
            top_k=self.top_k,
            filter_metadata=self.filter_metadata,
            score_threshold=self.score_threshold,
        )

        documents = []
        for result in results:
            doc = Document(
                page_content=result["content"],
                metadata={
                    **result["metadata"],
                    "relevance_score": result["score"],
                },
            )
            documents.append(doc)

        logger.debug(
            "Retrieved documents",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        return documents


class HybridRetriever(BaseRetriever):
    """Hybrid retriever combining semantic and keyword search."""

    semantic_weight: float = Field(default=0.7, description="Weight for semantic search")
    keyword_weight: float = Field(default=0.3, description="Weight for keyword search")
    top_k: int = Field(default=5, description="Number of documents to retrieve")

    class Config:
        arbitrary_types_allowed = True

    def _get_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Synchronous retrieval."""
        import asyncio

        loop = asyncio.get_event_loop()
        return loop.run_until_complete(
            self._aget_relevant_documents(query, run_manager=run_manager)
        )

    async def _aget_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Hybrid retrieval combining semantic and keyword search.

        Args:
            query: Search query.
            run_manager: Optional callback manager.

        Returns:
            List of relevant documents with combined scores.
        """
        vector_store = get_vector_store()

        # Semantic search
        semantic_results = await vector_store.search(
            query=query,
            top_k=self.top_k * 2,  # Get more for re-ranking
        )

        # Simple keyword matching boost
        query_terms = set(query.lower().split())

        # Re-rank with hybrid scoring
        scored_docs: dict[str, dict[str, Any]] = {}

        for result in semantic_results:
            content = result["content"]
            doc_id = result["metadata"].get("doc_id", "")

            # Semantic score (normalized)
            semantic_score = result["score"] * self.semantic_weight

            # Keyword score (simple term overlap)
            content_terms = set(content.lower().split())
            term_overlap = len(query_terms & content_terms) / max(len(query_terms), 1)
            keyword_score = term_overlap * self.keyword_weight

            combined_score = semantic_score + keyword_score

            key = f"{doc_id}_{result['metadata'].get('chunk_index', 0)}"
            scored_docs[key] = {
                "content": content,
                "metadata": result["metadata"],
                "score": combined_score,
            }

        # Sort by combined score and take top_k
        sorted_docs = sorted(
            scored_docs.values(),
            key=lambda x: x["score"],
            reverse=True,
        )[: self.top_k]

        documents = [
            Document(
                page_content=doc["content"],
                metadata={
                    **doc["metadata"],
                    "relevance_score": doc["score"],
                },
            )
            for doc in sorted_docs
        ]

        logger.debug(
            "Hybrid retrieval completed",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        return documents


class ContextualRetriever(BaseRetriever):
    """Retriever that adds contextual information to queries."""

    base_retriever: QdrantRetriever = Field(
        default_factory=QdrantRetriever,
        description="Base retriever to use",
    )
    conversation_context: list[str] = Field(
        default_factory=list,
        description="Previous conversation messages for context",
    )

    class Config:
        arbitrary_types_allowed = True

    def _get_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Synchronous retrieval."""
        import asyncio

        loop = asyncio.get_event_loop()
        return loop.run_until_complete(
            self._aget_relevant_documents(query, run_manager=run_manager)
        )

    async def _aget_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Retrieve with conversation context.

        Args:
            query: Current query.
            run_manager: Optional callback manager.

        Returns:
            Contextually relevant documents.
        """
        # Build enhanced query with context
        if self.conversation_context:
            # Take last few messages for context
            recent_context = self.conversation_context[-3:]
            context_str = " ".join(recent_context)
            enhanced_query = f"{context_str} {query}"
        else:
            enhanced_query = query

        return await self.base_retriever._aget_relevant_documents(
            enhanced_query,
            run_manager=run_manager,
        )

    def add_context(self, message: str) -> None:
        """Add a message to the conversation context.

        Args:
            message: Message to add.
        """
        self.conversation_context.append(message)
        # Keep only recent context
        if len(self.conversation_context) > 10:
            self.conversation_context = self.conversation_context[-10:]


class AdvancedRetriever(BaseRetriever):
    """Advanced retriever with reranking and context compression.

    Combines semantic search with cross-encoder reranking and
    optional context compression for improved relevance.
    """

    top_k: int = Field(default=5, description="Final number of documents to return")
    initial_k: int = Field(default=20, description="Initial retrieval count for reranking")
    use_reranker: bool = Field(default=True, description="Whether to use reranking")
    use_compressor: bool = Field(default=False, description="Whether to compress context")
    reranker_type: str = Field(default="cross-encoder", description="Type of reranker")
    compressor_type: str = Field(default="extractive", description="Type of compressor")

    class Config:
        arbitrary_types_allowed = True

    def _get_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Synchronous retrieval."""
        import asyncio

        loop = asyncio.get_event_loop()
        return loop.run_until_complete(
            self._aget_relevant_documents(query, run_manager=run_manager)
        )

    async def _aget_relevant_documents(
        self,
        query: str,
        *,
        run_manager: CallbackManagerForRetrieverRun | None = None,
    ) -> list[Document]:
        """Retrieve documents with reranking and compression.

        Args:
            query: Search query.
            run_manager: Optional callback manager.

        Returns:
            List of relevant, reranked, and optionally compressed documents.
        """
        from src.rag.retrieval.reranker import get_reranker
        from src.rag.retrieval.compressor import get_compressor

        vector_store = get_vector_store()

        # Step 1: Initial retrieval (get more docs for reranking)
        retrieval_k = self.initial_k if self.use_reranker else self.top_k

        results = await vector_store.search(
            query=query,
            top_k=retrieval_k,
        )

        documents = [
            Document(
                page_content=result["content"],
                metadata={
                    **result["metadata"],
                    "relevance_score": result["score"],
                },
            )
            for result in results
        ]

        logger.debug(
            "Initial retrieval complete",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        # Step 2: Reranking
        if self.use_reranker and documents:
            reranker = get_reranker(self.reranker_type, top_k=self.top_k)
            documents = await reranker.arerank(query, documents, top_k=self.top_k)
            logger.debug("Reranking complete", num_documents=len(documents))

        # Step 3: Context compression
        if self.use_compressor and documents:
            compressor = get_compressor(self.compressor_type)
            documents = await compressor.compress(query, documents)
            logger.debug("Compression complete", num_documents=len(documents))

        return documents


def get_retriever(
    retriever_type: str = "semantic",
    **kwargs: Any,
) -> BaseRetriever:
    """Factory function to get a retriever.

    Args:
        retriever_type: Type of retriever.
            - "semantic": Basic semantic search
            - "hybrid": Combined semantic + keyword search
            - "contextual": Semantic search with conversation context
            - "advanced": Semantic search with reranking and compression
        **kwargs: Additional arguments for the retriever.

    Returns:
        Configured retriever instance.
    """
    settings = get_settings()

    default_kwargs = {
        "top_k": settings.retrieval_top_k,
        "score_threshold": settings.retrieval_score_threshold,
    }
    default_kwargs.update(kwargs)

    if retriever_type == "hybrid":
        return HybridRetriever(**default_kwargs)
    elif retriever_type == "contextual":
        base = QdrantRetriever(**default_kwargs)
        return ContextualRetriever(base_retriever=base)
    elif retriever_type == "advanced":
        return AdvancedRetriever(
            top_k=default_kwargs.get("top_k", 5),
            use_reranker=settings.rerank_enabled,
            use_compressor=kwargs.get("use_compressor", False),
        )
    else:
        return QdrantRetriever(**default_kwargs)
