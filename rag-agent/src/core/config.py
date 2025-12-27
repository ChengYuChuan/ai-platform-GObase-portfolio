"""Application configuration using Pydantic Settings."""

from functools import lru_cache
from typing import Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_prefix="RAG_",
        case_sensitive=False,
        extra="ignore",
    )

    # Application
    app_name: str = "RAG Agent Service"
    app_version: str = "0.1.0"
    debug: bool = False
    environment: Literal["development", "staging", "production"] = "development"

    # Server
    host: str = "0.0.0.0"
    port: int = 8000
    workers: int = 1

    # LLM Providers
    openai_api_key: str = Field(default="", description="OpenAI API key")
    anthropic_api_key: str = Field(default="", description="Anthropic API key")
    default_llm_provider: Literal["openai", "anthropic"] = "openai"
    default_model: str = "gpt-4o-mini"

    # Embedding
    embedding_model: str = "text-embedding-3-small"
    embedding_dimension: int = 1536

    # Qdrant Vector Database
    qdrant_host: str = "localhost"
    qdrant_port: int = 6333
    qdrant_api_key: str = ""
    qdrant_collection: str = "documents"

    # Document Processing
    chunk_size: int = 1000
    chunk_overlap: int = 200
    max_document_size_mb: int = 50

    # RAG Pipeline
    retrieval_top_k: int = 5
    retrieval_score_threshold: float | None = None
    rerank_enabled: bool = False
    hybrid_search_enabled: bool = True
    hybrid_alpha: float = 0.5  # Balance between dense and sparse search
    llm_model: str = "gpt-4o-mini"

    # Agent Settings
    agent_max_iterations: int = 10
    agent_timeout_seconds: int = 120

    # Logging & Observability
    log_level: Literal["DEBUG", "INFO", "WARNING", "ERROR"] = "INFO"
    log_format: Literal["json", "console"] = "json"
    enable_tracing: bool = True
    otlp_endpoint: str = "http://localhost:4317"

    # Gateway Integration
    gateway_url: str = "http://localhost:8080"
    gateway_api_key: str = ""

    @property
    def is_production(self) -> bool:
        """Check if running in production environment."""
        return self.environment == "production"

    @property
    def qdrant_url(self) -> str:
        """Get Qdrant connection URL."""
        return f"http://{self.qdrant_host}:{self.qdrant_port}"


@lru_cache
def get_settings() -> Settings:
    """Get cached settings instance."""
    return Settings()


# Global settings instance
settings = get_settings()
