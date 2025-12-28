"""Document-related tools for agents."""

from typing import Any

from src.agents.tools.base import BaseTool, ToolResult
from src.core import get_logger
from src.core.config import get_settings
from src.rag.retrieval.vector_store import get_vector_store

logger = get_logger(__name__)


class SearchDocumentsTool(BaseTool):
    """Search for relevant documents in the knowledge base."""

    name = "search_documents"
    description = """Search for documents related to a query.
    Use this tool to find information from the document knowledge base.
    Returns a list of relevant document excerpts with their sources."""

    async def execute(
        self,
        query: str,
        top_k: int = 5,
        **kwargs: Any,
    ) -> ToolResult:
        """Search for documents.

        Args:
            query: Search query.
            top_k: Number of results to return.

        Returns:
            ToolResult with search results.
        """
        try:
            vector_store = get_vector_store()
            results = await vector_store.search(query=query, top_k=top_k)

            formatted_results = []
            for i, result in enumerate(results, 1):
                formatted_results.append({
                    "rank": i,
                    "content": result["content"],
                    "source": result["metadata"].get("filename", "Unknown"),
                    "score": result["score"],
                })

            return ToolResult.success(
                formatted_results,
                query=query,
                total_results=len(formatted_results),
            )

        except Exception as e:
            logger.error("Document search failed", error=str(e))
            return ToolResult.error(f"Search failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Search query to find relevant documents",
                    },
                    "top_k": {
                        "type": "integer",
                        "description": "Number of results to return",
                        "default": 5,
                    },
                },
                "required": ["query"],
            },
        }


class ReadDocumentTool(BaseTool):
    """Read the full content of a specific document."""

    name = "read_document"
    description = """Read the complete content of a document by its ID.
    Use this tool when you need the full text of a specific document."""

    async def execute(
        self,
        document_id: str,
        **kwargs: Any,
    ) -> ToolResult:
        """Read a document.

        Args:
            document_id: ID of the document to read.

        Returns:
            ToolResult with document content.
        """
        try:
            vector_store = get_vector_store()

            # Get all chunks for this document
            results = await vector_store.search(
                query="",  # Empty query to get all
                top_k=100,
                filter_metadata={"doc_id": document_id},
            )

            if not results:
                return ToolResult.error(f"Document not found: {document_id}")

            # Sort by chunk index and combine
            sorted_chunks = sorted(
                results,
                key=lambda x: x["metadata"].get("chunk_index", 0),
            )

            full_content = "\n\n".join(r["content"] for r in sorted_chunks)

            return ToolResult.success(
                {
                    "document_id": document_id,
                    "filename": sorted_chunks[0]["metadata"].get("filename", "Unknown"),
                    "content": full_content,
                    "chunks_count": len(sorted_chunks),
                },
            )

        except Exception as e:
            logger.error("Document read failed", error=str(e))
            return ToolResult.error(f"Read failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "document_id": {
                        "type": "string",
                        "description": "ID of the document to read",
                    },
                },
                "required": ["document_id"],
            },
        }


class SummarizeDocumentTool(BaseTool):
    """Summarize a document or text content."""

    name = "summarize_document"
    description = """Create a summary of document content or text.
    Use this tool to get a concise summary of lengthy content."""

    async def execute(
        self,
        content: str,
        max_length: int = 500,
        **kwargs: Any,
    ) -> ToolResult:
        """Summarize content.

        Args:
            content: Text content to summarize.
            max_length: Maximum length of summary.

        Returns:
            ToolResult with summary.
        """
        try:
            from langchain_openai import ChatOpenAI
            from langchain_core.prompts import ChatPromptTemplate

            settings = get_settings()

            llm = ChatOpenAI(
                model="gpt-4o-mini",
                temperature=0,
                openai_api_key=settings.openai_api_key,
            )

            prompt = ChatPromptTemplate.from_template(
                """Summarize the following content in a clear and concise manner.
Keep the summary under {max_length} characters.

Content:
{content}

Summary:"""
            )

            chain = prompt | llm
            result = await chain.ainvoke({
                "content": content[:10000],  # Limit input
                "max_length": max_length,
            })

            return ToolResult.success(
                {
                    "summary": result.content,
                    "original_length": len(content),
                    "summary_length": len(result.content),
                },
            )

        except Exception as e:
            logger.error("Summarization failed", error=str(e))
            return ToolResult.error(f"Summarization failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "content": {
                        "type": "string",
                        "description": "Text content to summarize",
                    },
                    "max_length": {
                        "type": "integer",
                        "description": "Maximum length of summary",
                        "default": 500,
                    },
                },
                "required": ["content"],
            },
        }


class ListDocumentsTool(BaseTool):
    """List all available documents in the knowledge base."""

    name = "list_documents"
    description = """List all documents available in the knowledge base.
    Use this tool to see what documents are available for search."""

    async def execute(
        self,
        limit: int = 20,
        **kwargs: Any,
    ) -> ToolResult:
        """List documents.

        Args:
            limit: Maximum number of documents to return.

        Returns:
            ToolResult with document list.
        """
        try:
            vector_store = get_vector_store()
            documents = await vector_store.list_documents(limit=limit)

            return ToolResult.success(
                {
                    "documents": documents,
                    "total": len(documents),
                },
            )

        except Exception as e:
            logger.error("Document list failed", error=str(e))
            return ToolResult.error(f"List failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of documents to return",
                        "default": 20,
                    },
                },
                "required": [],
            },
        }
