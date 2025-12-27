"""Document ingestion and management API endpoints."""

from typing import Annotated
from uuid import UUID, uuid4

from fastapi import APIRouter, File, HTTPException, Query, UploadFile
from pydantic import BaseModel, Field

from src.core import get_logger
from src.rag.ingestion.loader import DocumentLoader
from src.rag.retrieval.vector_store import get_vector_store

router = APIRouter()
logger = get_logger(__name__)


class DocumentMetadata(BaseModel):
    """Document metadata."""

    id: UUID = Field(default_factory=uuid4)
    filename: str
    content_type: str
    size_bytes: int
    chunks_count: int
    status: str = "processed"


class DocumentUploadResponse(BaseModel):
    """Response for document upload."""

    success: bool
    document: DocumentMetadata | None = None
    message: str


class DocumentListResponse(BaseModel):
    """Response for listing documents."""

    documents: list[DocumentMetadata]
    total: int


class DocumentSearchRequest(BaseModel):
    """Request for document search."""

    query: str = Field(..., min_length=1, max_length=1000)
    top_k: int = Field(default=5, ge=1, le=20)
    filter_metadata: dict | None = None


class SearchResult(BaseModel):
    """Single search result."""

    content: str
    score: float
    metadata: dict


class DocumentSearchResponse(BaseModel):
    """Response for document search."""

    results: list[SearchResult]
    query: str
    total: int


@router.post("/upload", response_model=DocumentUploadResponse)
async def upload_document(
    file: Annotated[UploadFile, File(description="Document file (PDF, DOCX, MD, TXT)")],
) -> DocumentUploadResponse:
    """Upload and process a document for RAG.

    Supports:
    - PDF files (.pdf)
    - Word documents (.docx)
    - Markdown files (.md)
    - Plain text files (.txt)
    """
    if not file.filename:
        raise HTTPException(status_code=400, detail="Filename is required")

    # Validate file type
    allowed_types = {
        "application/pdf": ".pdf",
        "application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
        "text/markdown": ".md",
        "text/plain": ".txt",
    }

    content_type = file.content_type or ""
    extension = file.filename.lower().split(".")[-1] if "." in file.filename else ""

    if content_type not in allowed_types and extension not in ["pdf", "docx", "md", "txt"]:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported file type. Allowed: PDF, DOCX, MD, TXT",
        )

    try:
        # Read file content
        content = await file.read()
        size_bytes = len(content)

        logger.info(
            "Processing document upload",
            filename=file.filename,
            size_bytes=size_bytes,
            content_type=content_type,
        )

        # Process document
        loader = DocumentLoader()
        chunks = await loader.load_and_split(
            content=content,
            filename=file.filename,
            content_type=content_type,
        )

        # Store in vector database
        vector_store = get_vector_store()
        doc_id = uuid4()

        await vector_store.add_documents(
            documents=chunks,
            doc_id=str(doc_id),
            metadata={"filename": file.filename, "content_type": content_type},
        )

        document = DocumentMetadata(
            id=doc_id,
            filename=file.filename,
            content_type=content_type or "application/octet-stream",
            size_bytes=size_bytes,
            chunks_count=len(chunks),
        )

        logger.info(
            "Document processed successfully",
            doc_id=str(doc_id),
            chunks=len(chunks),
        )

        return DocumentUploadResponse(
            success=True,
            document=document,
            message=f"Document processed into {len(chunks)} chunks",
        )

    except Exception as e:
        logger.error("Failed to process document", error=str(e), exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to process document: {str(e)}")


@router.get("/", response_model=DocumentListResponse)
async def list_documents(
    limit: Annotated[int, Query(ge=1, le=100)] = 20,
    offset: Annotated[int, Query(ge=0)] = 0,
) -> DocumentListResponse:
    """List all uploaded documents."""
    try:
        vector_store = get_vector_store()
        documents = await vector_store.list_documents(limit=limit, offset=offset)
        total = await vector_store.count_documents()

        return DocumentListResponse(documents=documents, total=total)
    except Exception as e:
        logger.error("Failed to list documents", error=str(e))
        raise HTTPException(status_code=500, detail="Failed to list documents")


@router.delete("/{document_id}")
async def delete_document(document_id: UUID) -> dict:
    """Delete a document and its vectors."""
    try:
        vector_store = get_vector_store()
        await vector_store.delete_document(str(document_id))

        logger.info("Document deleted", doc_id=str(document_id))
        return {"success": True, "message": f"Document {document_id} deleted"}

    except Exception as e:
        logger.error("Failed to delete document", error=str(e))
        raise HTTPException(status_code=500, detail=f"Failed to delete document: {str(e)}")


@router.post("/search", response_model=DocumentSearchResponse)
async def search_documents(request: DocumentSearchRequest) -> DocumentSearchResponse:
    """Search documents using semantic similarity."""
    try:
        vector_store = get_vector_store()
        results = await vector_store.search(
            query=request.query,
            top_k=request.top_k,
            filter_metadata=request.filter_metadata,
        )

        search_results = [
            SearchResult(
                content=r["content"],
                score=r["score"],
                metadata=r["metadata"],
            )
            for r in results
        ]

        return DocumentSearchResponse(
            results=search_results,
            query=request.query,
            total=len(search_results),
        )

    except Exception as e:
        logger.error("Search failed", error=str(e), query=request.query)
        raise HTTPException(status_code=500, detail=f"Search failed: {str(e)}")
