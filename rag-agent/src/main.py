"""RAG Agent Service - Main FastAPI Application."""

from contextlib import asynccontextmanager
from typing import AsyncGenerator

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from src.api import router as api_router
from src.core import get_logger, settings

logger = get_logger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Application lifespan handler for startup and shutdown events."""
    # Startup
    logger.info(
        "Starting RAG Agent Service",
        host=settings.host,
        port=settings.port,
        environment=settings.environment,
    )

    # Initialize services
    try:
        # Import here to avoid circular imports
        from src.rag.retrieval.vector_store import get_vector_store

        vector_store = get_vector_store()
        await vector_store.initialize()
        logger.info("Vector store initialized")
    except Exception as e:
        logger.warning("Failed to initialize vector store", error=str(e))

    yield

    # Shutdown
    logger.info("Shutting down RAG Agent Service")


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    app = FastAPI(
        title=settings.app_name,
        version=settings.app_version,
        description="Retrieval-Augmented Generation with Autonomous Agents",
        docs_url="/docs" if settings.debug else None,
        redoc_url="/redoc" if settings.debug else None,
        lifespan=lifespan,
    )

    # CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"] if settings.debug else ["http://localhost:3000"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Exception handlers
    @app.exception_handler(Exception)
    async def global_exception_handler(request: Request, exc: Exception) -> JSONResponse:
        """Global exception handler."""
        logger.error(
            "Unhandled exception",
            path=request.url.path,
            method=request.method,
            error=str(exc),
            exc_info=True,
        )
        return JSONResponse(
            status_code=500,
            content={"error": "Internal server error", "detail": str(exc) if settings.debug else None},
        )

    # Health check endpoints
    @app.get("/health")
    async def health_check() -> dict:
        """Health check endpoint."""
        return {
            "status": "healthy",
            "service": "rag-agent",
            "version": settings.app_version,
        }

    @app.get("/ready")
    async def readiness_check() -> dict:
        """Readiness check endpoint."""
        # Check dependencies
        checks = {
            "vector_store": False,
            "llm": False,
        }

        try:
            from src.rag.retrieval.vector_store import get_vector_store
            vector_store = get_vector_store()
            checks["vector_store"] = await vector_store.health_check()
        except Exception:
            pass

        try:
            if settings.openai_api_key or settings.anthropic_api_key:
                checks["llm"] = True
        except Exception:
            pass

        all_ready = all(checks.values())
        return {
            "status": "ready" if all_ready else "not_ready",
            "checks": checks,
        }

    # Include API routers
    app.include_router(api_router, prefix="/api/v1")

    return app


# Create the app instance
app = create_app()


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "src.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
        workers=settings.workers if not settings.debug else 1,
    )
