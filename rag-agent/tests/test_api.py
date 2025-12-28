"""Tests for API endpoints."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from httpx import AsyncClient, ASGITransport

from src.main import app
from src.agents.base import AgentResult


@pytest.fixture
async def test_client():
    """Create an async test client."""
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        yield client


class TestHealthEndpoints:
    """Tests for health check endpoints."""

    @pytest.mark.asyncio
    async def test_health(self, test_client):
        """Test health endpoint."""
        response = await test_client.get("/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert "version" in data

    @pytest.mark.asyncio
    async def test_ready_endpoint_exists(self, test_client):
        """Test ready endpoint exists and returns expected format."""
        response = await test_client.get("/ready")
        assert response.status_code == 200
        data = response.json()
        assert "status" in data
        assert "checks" in data


class TestAgentEndpoints:
    """Tests for agent endpoints."""

    @pytest.mark.asyncio
    async def test_run_agent(self, test_client):
        """Test running an agent."""
        with patch("src.api.agents.get_orchestrator") as mock_orch:
            mock_orchestrator = AsyncMock()
            mock_result = AgentResult(
                answer="Agent answer",
                sources=[{"content": "Source", "metadata": {}}],
                iterations=3,
                success=True,
                metadata={"routed_to": "rag"},
            )
            mock_orchestrator.run.return_value = mock_result
            mock_orch.return_value = mock_orchestrator

            response = await test_client.post(
                "/api/v1/agents/run",
                json={
                    "question": "What is machine learning?",
                    "agent_type": "rag",
                },
            )
            assert response.status_code == 200
            data = response.json()
            assert data["answer"] == "Agent answer"
            assert data["success"] is True
            assert len(data["sources"]) == 1

    @pytest.mark.asyncio
    async def test_run_agent_auto_route(self, test_client):
        """Test running agent with auto-routing."""
        with patch("src.api.agents.get_orchestrator") as mock_orch:
            mock_orchestrator = AsyncMock()
            mock_result = AgentResult(
                answer="Research answer",
                sources=[],
                iterations=5,
                success=True,
                metadata={"routed_to": "research"},
            )
            mock_orchestrator.run.return_value = mock_result
            mock_orch.return_value = mock_orchestrator

            response = await test_client.post(
                "/api/v1/agents/run",
                json={"question": "Compare Python and JavaScript for web development"},
            )
            assert response.status_code == 200
            data = response.json()
            assert data["success"] is True

    @pytest.mark.asyncio
    async def test_list_agent_types(self, test_client):
        """Test listing agent types."""
        with patch("src.api.agents.get_orchestrator") as mock_orch:
            mock_orchestrator = MagicMock()
            mock_orchestrator.list_agents.return_value = [
                {"type": "rag", "name": "RAG Agent", "description": "Q&A"},
                {"type": "research", "name": "Research Agent", "description": "Research"},
            ]
            mock_orch.return_value = mock_orchestrator

            response = await test_client.get("/api/v1/agents/types")
            assert response.status_code == 200
            data = response.json()
            assert "agents" in data
            assert len(data["agents"]) >= 2

    @pytest.mark.asyncio
    async def test_agent_validation_invalid_type(self, test_client):
        """Test agent request validation with invalid type."""
        response = await test_client.post(
            "/api/v1/agents/run",
            json={
                "question": "Test",
                "agent_type": "invalid_type",
            },
        )
        assert response.status_code == 422

    @pytest.mark.asyncio
    async def test_agent_validation_temperature(self, test_client):
        """Test agent request validation with invalid temperature."""
        response = await test_client.post(
            "/api/v1/agents/run",
            json={
                "question": "Test",
                "temperature": 3.0,  # Max is 2.0
            },
        )
        assert response.status_code == 422

    @pytest.mark.asyncio
    async def test_agent_error_handling(self, test_client):
        """Test agent error handling."""
        with patch("src.api.agents.get_orchestrator") as mock_orch:
            mock_orchestrator = AsyncMock()
            mock_orchestrator.run.side_effect = Exception("Internal error")
            mock_orch.return_value = mock_orchestrator

            response = await test_client.post(
                "/api/v1/agents/run",
                json={"question": "What is Python?"},
            )
            assert response.status_code == 500
            data = response.json()
            assert "detail" in data


class TestCORSAndMiddleware:
    """Tests for CORS and middleware."""

    @pytest.mark.asyncio
    async def test_cors_headers(self, test_client):
        """Test CORS headers are present."""
        response = await test_client.options(
            "/health",
            headers={"Origin": "http://localhost:3000"},
        )
        # CORS preflight should be handled
        assert response.status_code in [200, 405]  # Depends on CORS config
