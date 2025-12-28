"""Tests for Agent Orchestrator module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from src.agents.orchestrator import (
    AgentType,
    AgentOrchestrator,
    get_orchestrator,
)
from src.agents.base import AgentResult


class TestAgentType:
    """Tests for AgentType enum."""

    def test_agent_types(self):
        """Test agent type values."""
        assert AgentType.RAG.value == "rag"
        assert AgentType.RESEARCH.value == "research"
        assert AgentType.DATA_ENTRY.value == "data_entry"
        assert AgentType.SUPPORT_TRIAGE.value == "support_triage"
        assert AgentType.REPORT.value == "report"

    def test_agent_type_count(self):
        """Test number of agent types."""
        assert len(AgentType) == 5


class TestAgentOrchestrator:
    """Tests for AgentOrchestrator class."""

    def test_init_default(self):
        """Test initialization with default parameters."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"):
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            orchestrator = AgentOrchestrator()

            assert orchestrator.model_name == "gpt-4o-mini"
            assert orchestrator._agents == {}

    def test_init_custom_model(self):
        """Test initialization with custom model."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"):
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            orchestrator = AgentOrchestrator(model_name="gpt-4")

            assert orchestrator.model_name == "gpt-4"

    def test_get_agent_rag(self):
        """Test getting RAG agent."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_agent = MagicMock()
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()
            agent = orchestrator._get_agent("rag")

            assert agent is mock_agent
            mock_create.assert_called_once()

    def test_get_agent_caching(self):
        """Test agent caching."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_agent = MagicMock()
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()

            # Get same agent twice
            agent1 = orchestrator._get_agent("rag")
            agent2 = orchestrator._get_agent("rag")

            assert agent1 is agent2
            # Should only create once
            mock_create.assert_called_once()

    def test_get_agent_unknown_type(self):
        """Test getting unknown agent type."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"):
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            orchestrator = AgentOrchestrator()

            with pytest.raises(ValueError, match="Unknown agent type"):
                orchestrator._get_agent("nonexistent")

    @pytest.mark.asyncio
    async def test_route_request_rag(self):
        """Test routing to RAG agent."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI") as mock_chat:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_llm = AsyncMock()
            mock_response = MagicMock()
            mock_response.content = "rag"
            mock_llm.ainvoke = AsyncMock(return_value=mock_response)

            mock_chat.return_value = mock_llm

            orchestrator = AgentOrchestrator()
            # Override router_llm behavior
            orchestrator.router_llm = mock_llm

            # We need to mock the chain result
            with patch.object(orchestrator, 'router_llm', mock_llm):
                # Patch ChatPromptTemplate chain
                with patch("src.agents.orchestrator.ChatPromptTemplate") as mock_prompt:
                    mock_chain = AsyncMock()
                    mock_chain.ainvoke.return_value = mock_response
                    mock_prompt.from_template.return_value.__or__ = MagicMock(return_value=mock_chain)

                    result = await orchestrator.route_request("What is Python?")

                    assert result == "rag"

    @pytest.mark.asyncio
    async def test_route_request_invalid_defaults_to_rag(self):
        """Test that invalid routing result defaults to RAG."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.ChatPromptTemplate") as mock_prompt:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_response = MagicMock()
            mock_response.content = "invalid_agent_type"

            mock_chain = AsyncMock()
            mock_chain.ainvoke.return_value = mock_response
            mock_prompt.from_template.return_value.__or__ = MagicMock(return_value=mock_chain)

            orchestrator = AgentOrchestrator()
            result = await orchestrator.route_request("Some request")

            assert result == "rag"

    @pytest.mark.asyncio
    async def test_run_with_specified_agent(self):
        """Test running with specified agent type."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_agent = AsyncMock()
            mock_result = AgentResult(
                answer="Test answer",
                sources=[],
                iterations=1,
                success=True,
                metadata={},
            )
            mock_agent.run.return_value = mock_result
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()
            result = await orchestrator.run("What is Python?", agent_type="rag")

            assert result.answer == "Test answer"
            assert result.success is True
            assert result.metadata["routed_to"] == "rag"
            mock_agent.run.assert_called_once_with(question="What is Python?")

    @pytest.mark.asyncio
    async def test_run_with_auto_routing(self):
        """Test running with auto-routing."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_agent = AsyncMock()
            mock_result = AgentResult(
                answer="Routed answer",
                sources=[],
                iterations=1,
                success=True,
                metadata={},
            )
            mock_agent.run.return_value = mock_result
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()

            # Mock route_request
            orchestrator.route_request = AsyncMock(return_value="rag")

            result = await orchestrator.run("What is Python?")

            orchestrator.route_request.assert_called_once()
            assert result.success is True

    @pytest.mark.asyncio
    async def test_run_support_triage(self):
        """Test running support triage agent with ticket parameter."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_support_triage_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_agent = AsyncMock()
            mock_result = AgentResult(
                answer="Ticket triaged",
                sources=[],
                iterations=1,
                success=True,
                metadata={},
            )
            mock_agent.run.return_value = mock_result
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()
            result = await orchestrator.run(
                "My login is broken",
                agent_type="support_triage",
            )

            mock_agent.run.assert_called_once_with(ticket="My login is broken")
            assert result.success is True

    @pytest.mark.asyncio
    async def test_run_handles_exception(self):
        """Test that run handles agent exceptions gracefully."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_agent = AsyncMock()
            mock_agent.run.side_effect = Exception("Agent failed")
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()
            result = await orchestrator.run("What is Python?", agent_type="rag")

            assert result.success is False
            assert "Agent failed" in result.error
            assert result.metadata["routed_to"] == "rag"

    @pytest.mark.asyncio
    async def test_run_workflow(self):
        """Test running a multi-agent workflow."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_rag, \
             patch("src.agents.orchestrator.create_research_agent") as mock_research:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            # Mock RAG agent
            mock_rag_agent = AsyncMock()
            mock_rag_agent.run.return_value = AgentResult(
                answer="RAG result",
                sources=[],
                iterations=1,
                success=True,
                metadata={},
            )
            mock_rag.return_value = mock_rag_agent

            # Mock Research agent
            mock_research_agent = AsyncMock()
            mock_research_agent.run.return_value = AgentResult(
                answer="Research result",
                sources=[],
                iterations=2,
                success=True,
                metadata={},
            )
            mock_research.return_value = mock_research_agent

            orchestrator = AgentOrchestrator()

            workflow = [
                {"agent_type": "rag", "request": "Find info about X"},
                {"agent_type": "research", "request": "Analyze X", "use_previous": True},
            ]

            results = await orchestrator.run_workflow(workflow)

            assert len(results) == 2
            assert results[0].answer == "RAG result"
            assert results[0].metadata["workflow_step"] == 1
            assert results[1].answer == "Research result"
            assert results[1].metadata["workflow_step"] == 2

    @pytest.mark.asyncio
    async def test_run_workflow_stop_on_failure(self):
        """Test workflow stops on failure when specified."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_rag:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            mock_rag_agent = AsyncMock()
            mock_rag_agent.run.return_value = AgentResult(
                answer="",
                sources=[],
                iterations=1,
                success=False,
                error="Failed",
                metadata={},
            )
            mock_rag.return_value = mock_rag_agent

            orchestrator = AgentOrchestrator()

            workflow = [
                {"agent_type": "rag", "request": "Step 1", "stop_on_failure": True},
                {"agent_type": "rag", "request": "Step 2"},
            ]

            results = await orchestrator.run_workflow(workflow)

            # Should only have 1 result since we stopped on failure
            assert len(results) == 1
            assert results[0].success is False

    @pytest.mark.asyncio
    async def test_stream(self):
        """Test streaming agent execution."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            async def mock_stream(*args, **kwargs):
                yield {"event": "thinking", "data": "..."}
                yield {"event": "answer", "data": "Result"}

            mock_agent = MagicMock()
            mock_agent.stream = mock_stream
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()
            events = []
            async for event in orchestrator.stream("Test query", agent_type="rag"):
                events.append(event)

            assert len(events) == 2
            assert events[0]["agent_type"] == "rag"

    @pytest.mark.asyncio
    async def test_stream_with_auto_routing(self):
        """Test streaming with auto-routing."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"), \
             patch("src.agents.orchestrator.create_rag_agent") as mock_create:
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            async def mock_stream(*args, **kwargs):
                yield {"event": "done", "data": "Complete"}

            mock_agent = MagicMock()
            mock_agent.stream = mock_stream
            mock_create.return_value = mock_agent

            orchestrator = AgentOrchestrator()
            orchestrator.route_request = AsyncMock(return_value="rag")

            events = []
            async for event in orchestrator.stream("Test query"):
                events.append(event)

            # Should have routing event first
            assert events[0]["event"] == "routing"
            assert events[0]["agent_type"] == "rag"

    def test_list_agents(self):
        """Test listing available agents."""
        with patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"):
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            orchestrator = AgentOrchestrator()
            agents = orchestrator.list_agents()

            assert len(agents) == 5
            agent_types = [a["type"] for a in agents]
            assert "rag" in agent_types
            assert "research" in agent_types
            assert "data_entry" in agent_types
            assert "support_triage" in agent_types
            assert "report" in agent_types

            # Check structure
            for agent in agents:
                assert "type" in agent
                assert "name" in agent
                assert "description" in agent


class TestGetOrchestrator:
    """Tests for get_orchestrator function."""

    def test_get_orchestrator_singleton(self):
        """Test that get_orchestrator returns singleton."""
        with patch("src.agents.orchestrator._orchestrator", None), \
             patch("src.agents.orchestrator.get_settings") as mock_settings, \
             patch("src.agents.orchestrator.ChatOpenAI"):
            mock_settings.return_value.llm_model = "gpt-4o-mini"
            mock_settings.return_value.openai_api_key = "test-key"

            # Reset global
            import src.agents.orchestrator as orch_module
            orch_module._orchestrator = None

            orchestrator1 = get_orchestrator()
            orchestrator2 = get_orchestrator()

            assert orchestrator1 is orchestrator2
