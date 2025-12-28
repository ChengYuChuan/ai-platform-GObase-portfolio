"""Agent Orchestrator for coordinating multiple agents.

The orchestrator routes requests to appropriate agents and can
coordinate multi-agent workflows for complex tasks.
"""

from typing import Any, AsyncIterator
from enum import Enum

from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI

from src.agents.base import AgentConfig, AgentResult, BaseAgent
from src.agents.rag_agent import create_rag_agent
from src.agents.research_agent import create_research_agent
from src.agents.business.data_entry_agent import create_data_entry_agent
from src.agents.business.support_triage_agent import create_support_triage_agent
from src.agents.business.report_agent import create_report_agent
from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class AgentType(str, Enum):
    """Available agent types."""

    RAG = "rag"
    RESEARCH = "research"
    DATA_ENTRY = "data_entry"
    SUPPORT_TRIAGE = "support_triage"
    REPORT = "report"


class AgentOrchestrator:
    """Orchestrator for coordinating multiple agents.

    The orchestrator can:
    - Automatically route requests to appropriate agents
    - Coordinate multi-agent workflows
    - Aggregate results from multiple agents
    - Handle fallbacks and retries
    """

    def __init__(self, model_name: str | None = None) -> None:
        """Initialize the orchestrator.

        Args:
            model_name: LLM model for routing decisions.
        """
        self.settings = get_settings()
        self.model_name = model_name or self.settings.llm_model

        # Initialize routing LLM
        self.router_llm = ChatOpenAI(
            model=self.model_name,
            temperature=0,
            openai_api_key=self.settings.openai_api_key,
        )

        # Agent registry
        self._agents: dict[str, BaseAgent] = {}

    def _get_agent(self, agent_type: str) -> BaseAgent:
        """Get or create an agent of the specified type.

        Args:
            agent_type: Type of agent to get.

        Returns:
            Agent instance.
        """
        if agent_type not in self._agents:
            if agent_type == AgentType.RAG.value:
                self._agents[agent_type] = create_rag_agent()
            elif agent_type == AgentType.RESEARCH.value:
                self._agents[agent_type] = create_research_agent()
            elif agent_type == AgentType.DATA_ENTRY.value:
                self._agents[agent_type] = create_data_entry_agent()
            elif agent_type == AgentType.SUPPORT_TRIAGE.value:
                self._agents[agent_type] = create_support_triage_agent()
            elif agent_type == AgentType.REPORT.value:
                self._agents[agent_type] = create_report_agent()
            else:
                raise ValueError(f"Unknown agent type: {agent_type}")

        return self._agents[agent_type]

    async def route_request(self, request: str) -> str:
        """Determine which agent should handle a request.

        Args:
            request: User request.

        Returns:
            Agent type string.
        """
        prompt = ChatPromptTemplate.from_template(
            """Analyze this request and determine which agent should handle it.

Available agents:
- rag: Simple question answering about documents
- research: Complex research requiring multiple steps and synthesis
- data_entry: Extracting and validating structured data from documents
- support_triage: Customer support ticket classification and response
- report: Generating formatted reports from information

Request: {request}

Respond with only the agent name (rag, research, data_entry, support_triage, or report):"""
        )

        result = await (prompt | self.router_llm).ainvoke({"request": request})
        agent_type = result.content.strip().lower()

        # Validate agent type
        valid_types = {t.value for t in AgentType}
        if agent_type not in valid_types:
            logger.warning(
                "Invalid agent type returned, defaulting to rag",
                returned=agent_type,
            )
            agent_type = AgentType.RAG.value

        logger.info("Request routed", agent_type=agent_type, request_preview=request[:50])
        return agent_type

    async def run(
        self,
        request: str,
        agent_type: str | None = None,
        **kwargs: Any,
    ) -> AgentResult:
        """Run a request through the appropriate agent.

        Args:
            request: User request.
            agent_type: Optional agent type (auto-routes if not specified).
            **kwargs: Additional arguments for the agent.

        Returns:
            AgentResult from the selected agent.
        """
        logger.info("Orchestrator processing request", request_preview=request[:50])

        # Route request if agent type not specified
        if not agent_type:
            agent_type = await self.route_request(request)

        # Get and run the agent
        agent = self._get_agent(agent_type)

        try:
            # Different agents have different parameter names
            if agent_type == AgentType.SUPPORT_TRIAGE.value:
                result = await agent.run(ticket=request, **kwargs)
            elif agent_type == AgentType.DATA_ENTRY.value:
                result = await agent.run(request=request, **kwargs)
            elif agent_type == AgentType.REPORT.value:
                result = await agent.run(request=request, **kwargs)
            else:
                result = await agent.run(question=request, **kwargs)

            # Add routing metadata
            result.metadata["routed_to"] = agent_type

            return result

        except Exception as e:
            logger.error(
                "Agent execution failed",
                agent_type=agent_type,
                error=str(e),
            )
            return AgentResult(
                answer="",
                sources=[],
                iterations=0,
                success=False,
                error=str(e),
                metadata={"routed_to": agent_type},
            )

    async def run_workflow(
        self,
        workflow: list[dict[str, Any]],
    ) -> list[AgentResult]:
        """Run a multi-agent workflow.

        Args:
            workflow: List of workflow steps, each with:
                - agent_type: Type of agent to use
                - request: Request for the agent
                - use_previous: Whether to incorporate previous results
                - kwargs: Additional arguments

        Returns:
            List of results from each workflow step.
        """
        logger.info("Starting workflow", num_steps=len(workflow))

        results = []
        previous_result: AgentResult | None = None

        for i, step in enumerate(workflow):
            agent_type = step.get("agent_type")
            request = step.get("request", "")
            use_previous = step.get("use_previous", False)
            kwargs = step.get("kwargs", {})

            # Incorporate previous result if requested
            if use_previous and previous_result and previous_result.answer:
                request = f"{request}\n\nContext from previous step:\n{previous_result.answer}"

            logger.debug(
                "Executing workflow step",
                step=i + 1,
                agent_type=agent_type,
            )

            result = await self.run(
                request=request,
                agent_type=agent_type,
                **kwargs,
            )

            result.metadata["workflow_step"] = i + 1
            results.append(result)
            previous_result = result

            # Stop on failure if specified
            if not result.success and step.get("stop_on_failure", False):
                logger.warning("Workflow stopped due to failure", step=i + 1)
                break

        return results

    async def stream(
        self,
        request: str,
        agent_type: str | None = None,
        **kwargs: Any,
    ) -> AsyncIterator[dict[str, Any]]:
        """Stream agent execution.

        Args:
            request: User request.
            agent_type: Optional agent type.
            **kwargs: Additional arguments.

        Yields:
            Intermediate states and events.
        """
        # Route request if needed
        if not agent_type:
            agent_type = await self.route_request(request)
            yield {"event": "routing", "agent_type": agent_type}

        agent = self._get_agent(agent_type)

        # Stream from the agent
        if agent_type == AgentType.SUPPORT_TRIAGE.value:
            stream = agent.stream(ticket=request, **kwargs)
        elif agent_type in [AgentType.DATA_ENTRY.value, AgentType.REPORT.value]:
            stream = agent.stream(request=request, **kwargs)
        else:
            stream = agent.stream(question=request, **kwargs)

        async for event in stream:
            event["agent_type"] = agent_type
            yield event

    def list_agents(self) -> list[dict[str, str]]:
        """List available agents.

        Returns:
            List of agent information.
        """
        return [
            {
                "type": AgentType.RAG.value,
                "name": "RAG Agent",
                "description": "Question answering with document retrieval",
            },
            {
                "type": AgentType.RESEARCH.value,
                "name": "Research Agent",
                "description": "Multi-step research and synthesis",
            },
            {
                "type": AgentType.DATA_ENTRY.value,
                "name": "Data Entry Agent",
                "description": "Extract and validate structured data",
            },
            {
                "type": AgentType.SUPPORT_TRIAGE.value,
                "name": "Support Triage Agent",
                "description": "Classify and route support tickets",
            },
            {
                "type": AgentType.REPORT.value,
                "name": "Report Agent",
                "description": "Generate formatted reports",
            },
        ]


# Global orchestrator instance
_orchestrator: AgentOrchestrator | None = None


def get_orchestrator() -> AgentOrchestrator:
    """Get the global orchestrator instance.

    Returns:
        AgentOrchestrator singleton.
    """
    global _orchestrator
    if _orchestrator is None:
        _orchestrator = AgentOrchestrator()
    return _orchestrator
