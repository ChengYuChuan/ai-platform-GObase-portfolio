"""API routes for RAG Agent Service."""

from fastapi import APIRouter

from src.api.documents import router as documents_router
from src.api.chat import router as chat_router
from src.api.agents import router as agents_router

router = APIRouter()

# Include sub-routers
router.include_router(documents_router, prefix="/documents", tags=["Documents"])
router.include_router(chat_router, prefix="/chat", tags=["Chat"])
router.include_router(agents_router, prefix="/agents", tags=["Agents"])

__all__ = ["router"]
