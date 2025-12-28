"""Reranking module using cross-encoder models."""

from typing import Any

from langchain_core.documents import Document

from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class CrossEncoderReranker:
    """Reranker using cross-encoder models for improved relevance scoring.

    Cross-encoders jointly encode query-document pairs to produce
    more accurate relevance scores than bi-encoder similarity.
    """

    def __init__(
        self,
        model_name: str = "cross-encoder/ms-marco-MiniLM-L-6-v2",
        top_k: int | None = None,
        batch_size: int = 32,
    ) -> None:
        """Initialize the cross-encoder reranker.

        Args:
            model_name: HuggingFace model name for cross-encoder.
            top_k: Number of documents to return after reranking.
            batch_size: Batch size for inference.
        """
        self.model_name = model_name
        self.top_k = top_k
        self.batch_size = batch_size
        self._model = None
        self._initialized = False

    def _initialize(self) -> None:
        """Lazy initialization of the cross-encoder model."""
        if self._initialized:
            return

        try:
            from sentence_transformers import CrossEncoder

            logger.info("Loading cross-encoder model", model=self.model_name)
            self._model = CrossEncoder(self.model_name)
            self._initialized = True
            logger.info("Cross-encoder model loaded successfully")

        except ImportError:
            logger.error(
                "sentence-transformers not installed. "
                "Install with: pip install sentence-transformers"
            )
            raise ImportError(
                "sentence-transformers is required for cross-encoder reranking"
            )

    def rerank(
        self,
        query: str,
        documents: list[Document],
        top_k: int | None = None,
    ) -> list[Document]:
        """Rerank documents based on query relevance.

        Args:
            query: Search query.
            documents: List of documents to rerank.
            top_k: Number of documents to return (overrides init value).

        Returns:
            Reranked list of documents with updated scores.
        """
        if not documents:
            return []

        self._initialize()

        k = top_k or self.top_k or len(documents)

        # Prepare query-document pairs
        pairs = [(query, doc.page_content) for doc in documents]

        # Get cross-encoder scores
        logger.debug(
            "Reranking documents",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        scores = self._model.predict(pairs, batch_size=self.batch_size)

        # Create scored documents
        scored_docs = list(zip(documents, scores))

        # Sort by score descending
        scored_docs.sort(key=lambda x: x[1], reverse=True)

        # Update metadata with rerank score and return top_k
        reranked = []
        for doc, score in scored_docs[:k]:
            new_doc = Document(
                page_content=doc.page_content,
                metadata={
                    **doc.metadata,
                    "rerank_score": float(score),
                    "original_score": doc.metadata.get("relevance_score", 0),
                },
            )
            reranked.append(new_doc)

        logger.debug(
            "Reranking complete",
            input_count=len(documents),
            output_count=len(reranked),
        )

        return reranked

    async def arerank(
        self,
        query: str,
        documents: list[Document],
        top_k: int | None = None,
    ) -> list[Document]:
        """Async wrapper for rerank (runs in thread pool).

        Args:
            query: Search query.
            documents: List of documents to rerank.
            top_k: Number of documents to return.

        Returns:
            Reranked list of documents.
        """
        import asyncio

        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(
            None,
            lambda: self.rerank(query, documents, top_k),
        )


class CohereReranker:
    """Reranker using Cohere's rerank API."""

    def __init__(
        self,
        api_key: str | None = None,
        model: str = "rerank-english-v3.0",
        top_k: int | None = None,
    ) -> None:
        """Initialize the Cohere reranker.

        Args:
            api_key: Cohere API key (uses env var if not provided).
            model: Cohere rerank model name.
            top_k: Number of documents to return.
        """
        self.api_key = api_key
        self.model = model
        self.top_k = top_k
        self._client = None

    def _get_client(self):
        """Get or create Cohere client."""
        if self._client is None:
            try:
                import cohere

                api_key = self.api_key or get_settings().cohere_api_key
                if not api_key:
                    raise ValueError("Cohere API key not configured")
                self._client = cohere.Client(api_key)
            except ImportError:
                raise ImportError("cohere package is required for Cohere reranking")
        return self._client

    async def arerank(
        self,
        query: str,
        documents: list[Document],
        top_k: int | None = None,
    ) -> list[Document]:
        """Rerank documents using Cohere API.

        Args:
            query: Search query.
            documents: List of documents to rerank.
            top_k: Number of documents to return.

        Returns:
            Reranked list of documents.
        """
        if not documents:
            return []

        k = top_k or self.top_k or len(documents)
        client = self._get_client()

        # Extract text content
        doc_texts = [doc.page_content for doc in documents]

        logger.debug(
            "Cohere reranking",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        # Call Cohere rerank API
        response = client.rerank(
            query=query,
            documents=doc_texts,
            top_n=k,
            model=self.model,
        )

        # Build reranked document list
        reranked = []
        for result in response.results:
            original_doc = documents[result.index]
            new_doc = Document(
                page_content=original_doc.page_content,
                metadata={
                    **original_doc.metadata,
                    "rerank_score": result.relevance_score,
                    "original_score": original_doc.metadata.get("relevance_score", 0),
                },
            )
            reranked.append(new_doc)

        return reranked


class LLMReranker:
    """Reranker using LLM for relevance scoring.

    Uses GPT or other LLMs to score document relevance.
    More expensive but can be more accurate for complex queries.
    """

    def __init__(
        self,
        model_name: str | None = None,
        top_k: int | None = None,
    ) -> None:
        """Initialize the LLM reranker.

        Args:
            model_name: LLM model to use for scoring.
            top_k: Number of documents to return.
        """
        settings = get_settings()
        self.model_name = model_name or settings.llm_model
        self.top_k = top_k
        self._llm = None

    def _get_llm(self):
        """Get or create LLM instance."""
        if self._llm is None:
            from langchain_openai import ChatOpenAI

            settings = get_settings()
            self._llm = ChatOpenAI(
                model=self.model_name,
                temperature=0,
                openai_api_key=settings.openai_api_key,
            )
        return self._llm

    async def arerank(
        self,
        query: str,
        documents: list[Document],
        top_k: int | None = None,
    ) -> list[Document]:
        """Rerank documents using LLM scoring.

        Args:
            query: Search query.
            documents: List of documents to rerank.
            top_k: Number of documents to return.

        Returns:
            Reranked list of documents.
        """
        if not documents:
            return []

        k = top_k or self.top_k or len(documents)
        llm = self._get_llm()

        SCORING_PROMPT = """Rate the relevance of this document to the query on a scale of 0-10.
Only respond with a single number.

Query: {query}

Document:
{document}

Relevance score (0-10):"""

        logger.debug(
            "LLM reranking",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        # Score each document
        scored_docs = []
        for doc in documents:
            prompt = SCORING_PROMPT.format(
                query=query,
                document=doc.page_content[:1000],  # Truncate for efficiency
            )

            try:
                response = await llm.ainvoke(prompt)
                score = float(response.content.strip())
                score = max(0, min(10, score))  # Clamp to 0-10
            except (ValueError, AttributeError):
                score = 5.0  # Default score on parsing failure

            scored_docs.append((doc, score))

        # Sort by score descending
        scored_docs.sort(key=lambda x: x[1], reverse=True)

        # Build reranked list
        reranked = []
        for doc, score in scored_docs[:k]:
            new_doc = Document(
                page_content=doc.page_content,
                metadata={
                    **doc.metadata,
                    "rerank_score": score / 10.0,  # Normalize to 0-1
                    "original_score": doc.metadata.get("relevance_score", 0),
                },
            )
            reranked.append(new_doc)

        return reranked


def get_reranker(
    reranker_type: str = "cross-encoder",
    **kwargs: Any,
) -> CrossEncoderReranker | CohereReranker | LLMReranker:
    """Factory function to get a reranker.

    Args:
        reranker_type: Type of reranker ("cross-encoder", "cohere", "llm").
        **kwargs: Additional arguments for the reranker.

    Returns:
        Configured reranker instance.
    """
    settings = get_settings()

    if reranker_type == "cohere":
        return CohereReranker(
            top_k=kwargs.get("top_k", settings.retrieval_top_k),
            **kwargs,
        )
    elif reranker_type == "llm":
        return LLMReranker(
            top_k=kwargs.get("top_k", settings.retrieval_top_k),
            **kwargs,
        )
    else:
        return CrossEncoderReranker(
            top_k=kwargs.get("top_k", settings.retrieval_top_k),
            **kwargs,
        )
