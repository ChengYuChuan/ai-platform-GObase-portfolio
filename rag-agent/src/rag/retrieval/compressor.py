"""Context compression module for RAG pipeline.

Compresses retrieved context to include only the most relevant
information, reducing token usage and improving response quality.
"""

from typing import Any

from langchain_core.documents import Document
from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI

from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class LLMContextCompressor:
    """Compress context using LLM to extract relevant information.

    This compressor uses an LLM to identify and extract only the
    parts of documents that are relevant to the query.
    """

    COMPRESSION_PROMPT = """Given the following document and question, extract only the parts of the document that are directly relevant to answering the question.
If no parts are relevant, respond with "NOT_RELEVANT".
Keep the extracted content concise but complete enough to answer the question.

Question: {question}

Document:
{document}

Relevant content:"""

    def __init__(
        self,
        model_name: str | None = None,
        max_tokens: int = 500,
    ) -> None:
        """Initialize the LLM context compressor.

        Args:
            model_name: LLM model to use for compression.
            max_tokens: Maximum tokens in compressed output per document.
        """
        settings = get_settings()
        self.model_name = model_name or "gpt-4o-mini"
        self.max_tokens = max_tokens
        self._llm = None

    def _get_llm(self) -> ChatOpenAI:
        """Get or create LLM instance."""
        if self._llm is None:
            settings = get_settings()
            self._llm = ChatOpenAI(
                model=self.model_name,
                temperature=0,
                max_tokens=self.max_tokens,
                openai_api_key=settings.openai_api_key,
            )
        return self._llm

    async def compress(
        self,
        query: str,
        documents: list[Document],
    ) -> list[Document]:
        """Compress documents to include only relevant content.

        Args:
            query: User query.
            documents: List of documents to compress.

        Returns:
            List of compressed documents.
        """
        if not documents:
            return []

        llm = self._get_llm()
        compressed_docs = []

        logger.debug(
            "Compressing context",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        for doc in documents:
            prompt = self.COMPRESSION_PROMPT.format(
                question=query,
                document=doc.page_content,
            )

            try:
                response = await llm.ainvoke(prompt)
                compressed_content = response.content.strip()

                # Skip if not relevant
                if compressed_content.upper() == "NOT_RELEVANT":
                    logger.debug(
                        "Document marked not relevant",
                        filename=doc.metadata.get("filename", "unknown"),
                    )
                    continue

                # Create compressed document
                compressed_doc = Document(
                    page_content=compressed_content,
                    metadata={
                        **doc.metadata,
                        "compressed": True,
                        "original_length": len(doc.page_content),
                        "compressed_length": len(compressed_content),
                    },
                )
                compressed_docs.append(compressed_doc)

            except Exception as e:
                logger.warning(
                    "Compression failed, using original",
                    error=str(e),
                )
                compressed_docs.append(doc)

        logger.debug(
            "Context compression complete",
            input_count=len(documents),
            output_count=len(compressed_docs),
        )

        return compressed_docs


class EmbeddingContextCompressor:
    """Compress context using embedding similarity.

    Splits documents into sentences and keeps only those with
    high similarity to the query.
    """

    def __init__(
        self,
        similarity_threshold: float = 0.7,
        max_sentences: int = 10,
    ) -> None:
        """Initialize the embedding context compressor.

        Args:
            similarity_threshold: Minimum similarity score to keep sentence.
            max_sentences: Maximum sentences to keep per document.
        """
        self.similarity_threshold = similarity_threshold
        self.max_sentences = max_sentences
        self._embeddings = None

    def _get_embeddings(self):
        """Get or create embeddings instance."""
        if self._embeddings is None:
            from langchain_openai import OpenAIEmbeddings

            settings = get_settings()
            self._embeddings = OpenAIEmbeddings(
                model=settings.embedding_model,
                openai_api_key=settings.openai_api_key,
            )
        return self._embeddings

    def _split_sentences(self, text: str) -> list[str]:
        """Split text into sentences."""
        import re

        # Simple sentence splitting
        sentences = re.split(r'(?<=[.!?])\s+', text)
        return [s.strip() for s in sentences if s.strip()]

    def _cosine_similarity(self, vec1: list[float], vec2: list[float]) -> float:
        """Calculate cosine similarity between two vectors."""
        import math

        dot_product = sum(a * b for a, b in zip(vec1, vec2))
        norm1 = math.sqrt(sum(a * a for a in vec1))
        norm2 = math.sqrt(sum(b * b for b in vec2))

        if norm1 == 0 or norm2 == 0:
            return 0.0

        return dot_product / (norm1 * norm2)

    async def compress(
        self,
        query: str,
        documents: list[Document],
    ) -> list[Document]:
        """Compress documents by keeping only relevant sentences.

        Args:
            query: User query.
            documents: List of documents to compress.

        Returns:
            List of compressed documents.
        """
        if not documents:
            return []

        embeddings = self._get_embeddings()

        logger.debug(
            "Embedding-based compression",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        # Get query embedding
        query_embedding = await embeddings.aembed_query(query)

        compressed_docs = []

        for doc in documents:
            sentences = self._split_sentences(doc.page_content)

            if not sentences:
                continue

            # Get sentence embeddings
            sentence_embeddings = await embeddings.aembed_documents(sentences)

            # Calculate similarities and filter
            scored_sentences = []
            for sentence, embedding in zip(sentences, sentence_embeddings):
                similarity = self._cosine_similarity(query_embedding, embedding)
                if similarity >= self.similarity_threshold:
                    scored_sentences.append((sentence, similarity))

            # Sort by similarity and take top sentences
            scored_sentences.sort(key=lambda x: x[1], reverse=True)
            top_sentences = scored_sentences[: self.max_sentences]

            if not top_sentences:
                # If no sentences pass threshold, keep original
                compressed_docs.append(doc)
                continue

            # Reconstruct compressed content
            compressed_content = " ".join(s for s, _ in top_sentences)

            compressed_doc = Document(
                page_content=compressed_content,
                metadata={
                    **doc.metadata,
                    "compressed": True,
                    "original_length": len(doc.page_content),
                    "compressed_length": len(compressed_content),
                    "kept_sentences": len(top_sentences),
                    "total_sentences": len(sentences),
                },
            )
            compressed_docs.append(compressed_doc)

        return compressed_docs


class ExtractiveSummaryCompressor:
    """Compress context using extractive summarization.

    Uses sentence scoring based on position, keyword overlap,
    and sentence length to select important sentences.
    """

    def __init__(
        self,
        compression_ratio: float = 0.3,
        min_sentences: int = 2,
        max_sentences: int = 10,
    ) -> None:
        """Initialize the extractive summary compressor.

        Args:
            compression_ratio: Target ratio of output to input length.
            min_sentences: Minimum sentences to keep.
            max_sentences: Maximum sentences to keep.
        """
        self.compression_ratio = compression_ratio
        self.min_sentences = min_sentences
        self.max_sentences = max_sentences

    def _split_sentences(self, text: str) -> list[str]:
        """Split text into sentences."""
        import re

        sentences = re.split(r'(?<=[.!?])\s+', text)
        return [s.strip() for s in sentences if s.strip()]

    def _score_sentence(
        self,
        sentence: str,
        position: int,
        total_sentences: int,
        query_terms: set[str],
    ) -> float:
        """Score a sentence based on various factors.

        Args:
            sentence: The sentence to score.
            position: Position in document (0-indexed).
            total_sentences: Total number of sentences.
            query_terms: Set of query terms for overlap scoring.

        Returns:
            Sentence score.
        """
        score = 0.0

        # Position score (first and last sentences often important)
        if position == 0:
            score += 0.3
        elif position == total_sentences - 1:
            score += 0.1
        else:
            # Middle sentences get slight penalty
            score += 0.1 * (1 - abs(position - total_sentences / 2) / total_sentences)

        # Query term overlap
        sentence_terms = set(sentence.lower().split())
        overlap = len(query_terms & sentence_terms)
        score += 0.4 * (overlap / max(len(query_terms), 1))

        # Length score (prefer medium-length sentences)
        word_count = len(sentence.split())
        if 10 <= word_count <= 30:
            score += 0.2
        elif word_count < 5:
            score -= 0.1

        return score

    async def compress(
        self,
        query: str,
        documents: list[Document],
    ) -> list[Document]:
        """Compress documents using extractive summarization.

        Args:
            query: User query.
            documents: List of documents to compress.

        Returns:
            List of compressed documents.
        """
        if not documents:
            return []

        query_terms = set(query.lower().split())
        compressed_docs = []

        logger.debug(
            "Extractive compression",
            query_preview=query[:50],
            num_documents=len(documents),
        )

        for doc in documents:
            sentences = self._split_sentences(doc.page_content)

            if len(sentences) <= self.min_sentences:
                compressed_docs.append(doc)
                continue

            # Score all sentences
            scored = [
                (
                    sentence,
                    self._score_sentence(
                        sentence,
                        i,
                        len(sentences),
                        query_terms,
                    ),
                )
                for i, sentence in enumerate(sentences)
            ]

            # Sort by score
            scored.sort(key=lambda x: x[1], reverse=True)

            # Determine number of sentences to keep
            target_count = max(
                self.min_sentences,
                min(
                    self.max_sentences,
                    int(len(sentences) * self.compression_ratio),
                ),
            )

            # Select top sentences
            top_sentences = scored[:target_count]

            # Sort back to original order for coherence
            original_order = {s: i for i, s in enumerate(sentences)}
            top_sentences.sort(key=lambda x: original_order.get(x[0], 0))

            # Reconstruct content
            compressed_content = " ".join(s for s, _ in top_sentences)

            compressed_doc = Document(
                page_content=compressed_content,
                metadata={
                    **doc.metadata,
                    "compressed": True,
                    "original_length": len(doc.page_content),
                    "compressed_length": len(compressed_content),
                    "kept_sentences": len(top_sentences),
                    "total_sentences": len(sentences),
                },
            )
            compressed_docs.append(compressed_doc)

        return compressed_docs


def get_compressor(
    compressor_type: str = "extractive",
    **kwargs: Any,
) -> LLMContextCompressor | EmbeddingContextCompressor | ExtractiveSummaryCompressor:
    """Factory function to get a context compressor.

    Args:
        compressor_type: Type of compressor ("llm", "embedding", "extractive").
        **kwargs: Additional arguments for the compressor.

    Returns:
        Configured compressor instance.
    """
    if compressor_type == "llm":
        return LLMContextCompressor(**kwargs)
    elif compressor_type == "embedding":
        return EmbeddingContextCompressor(**kwargs)
    else:
        return ExtractiveSummaryCompressor(**kwargs)
