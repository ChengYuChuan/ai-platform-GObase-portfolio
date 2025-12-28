"""Agent API endpoints for autonomous AI tasks."""

import asyncio
from typing import Annotated, Any, AsyncIterator
from uuid import uuid4

from fastapi import APIRouter, Body, HTTPException, Query
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from src.core import get_logger
from src.core.config import get_settings
from src.agents.rag_agent import create_rag_agent
from src.agents.research_agent import create_research_agent
from src.agents.orchestrator import get_orchestrator

router = APIRouter()
logger = get_logger(__name__)


class AgentRequest(BaseModel):
    """Request for agent execution."""

    question: str = Field(..., min_length=1, max_length=2000)
    agent_type: str | None = Field(
        default=None,
        pattern="^(rag|research|data_entry|support_triage|report)?$",
        description="Agent type. If not specified, auto-routes based on request.",
    )
    model: str | None = None
    temperature: float = Field(default=0.7, ge=0.0, le=2.0)
    max_iterations: int = Field(default=10, ge=1, le=50)
    stream: bool = False


class AgentSource(BaseModel):
    """Source document from agent research."""

    content: str
    metadata: dict[str, Any]


class AgentResponse(BaseModel):
    """Response from agent execution."""

    id: str
    agent_type: str
    answer: str
    sources: list[AgentSource]
    iterations: int
    success: bool
    error: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class AgentStatus(BaseModel):
    """Status update during agent execution."""

    event: str
    iteration: int
    status: str
    details: dict[str, Any] = Field(default_factory=dict)


@router.post("/run", response_model=AgentResponse)
async def run_agent(request: AgentRequest) -> AgentResponse | StreamingResponse:
    """Run an autonomous agent to answer a question.

    Available agent types:
    - `rag`: Basic RAG agent for document Q&A
    - `research`: Multi-step research agent for complex questions
    - `data_entry`: Extract structured data from documents
    - `support_triage`: Classify and route support tickets
    - `report`: Generate formatted reports

    If agent_type is not specified, the orchestrator will auto-route
    based on the request content.
    """
    logger.info(
        "Agent request",
        agent_type=request.agent_type,
        stream=request.stream,
        question_preview=request.question[:50],
    )

    try:
        if request.stream:
            return StreamingResponse(
                _stream_agent_response(request),
                media_type="text/event-stream",
            )

        # Use orchestrator for routing and execution
        orchestrator = get_orchestrator()
        result = await orchestrator.run(
            request=request.question,
            agent_type=request.agent_type,
        )

        response_id = str(uuid4())
        routed_agent_type = result.metadata.get("routed_to", request.agent_type or "rag")

        return AgentResponse(
            id=response_id,
            agent_type=routed_agent_type,
            answer=result.answer,
            sources=[
                AgentSource(
                    content=s.get("content", ""),
                    metadata=s.get("metadata", {}),
                )
                for s in result.sources
            ],
            iterations=result.iterations,
            success=result.success,
            error=result.error,
            metadata=result.metadata,
        )

    except Exception as e:
        logger.error("Agent execution failed", error=str(e), exc_info=True)
        raise HTTPException(status_code=500, detail=f"Agent execution failed: {str(e)}")


async def _stream_agent_response(request: AgentRequest) -> AsyncIterator[str]:
    """Stream agent execution as SSE events.

    Args:
        request: Agent request.

    Yields:
        SSE event strings.
    """
    import json

    response_id = str(uuid4())

    try:
        # Use orchestrator for streaming
        orchestrator = get_orchestrator()

        async for event in orchestrator.stream(
            request=request.question,
            agent_type=request.agent_type,
        ):
            event_data = {
                "id": response_id,
                **event,
            }
            yield f"data: {json.dumps(event_data)}\n\n"

        # Send done event
        yield "data: [DONE]\n\n"

    except Exception as e:
        logger.error("Agent stream failed", error=str(e))
        error_data = {"error": str(e)}
        yield f"data: {json.dumps(error_data)}\n\n"


@router.post("/rag", response_model=AgentResponse)
async def run_rag_agent(
    question: Annotated[str, Body(..., min_length=1, max_length=2000, embed=True)],
    model: Annotated[str | None, Body(embed=True)] = None,
    temperature: Annotated[float, Body(embed=True)] = 0.7,
    max_iterations: Annotated[int, Body(embed=True)] = 10,
) -> AgentResponse:
    """Run the RAG agent for document Q&A.

    This is a simplified endpoint specifically for RAG queries.
    """
    request = AgentRequest(
        question=question,
        agent_type="rag",
        model=model,
        temperature=temperature,
        max_iterations=max_iterations,
    )

    result = await run_agent(request)

    # Handle StreamingResponse case (shouldn't happen here but for type safety)
    if isinstance(result, StreamingResponse):
        raise HTTPException(status_code=500, detail="Unexpected streaming response")

    return result


@router.post("/research", response_model=AgentResponse)
async def run_research_agent(
    question: Annotated[str, Body(..., min_length=1, max_length=2000, embed=True)],
    model: Annotated[str | None, Body(embed=True)] = None,
    temperature: Annotated[float, Body(embed=True)] = 0.7,
    max_iterations: Annotated[int, Body(embed=True)] = 15,
) -> AgentResponse:
    """Run the research agent for complex multi-step research.

    The research agent:
    1. Creates a research plan
    2. Executes each research step
    3. Synthesizes findings into a comprehensive answer
    """
    request = AgentRequest(
        question=question,
        agent_type="research",
        model=model,
        temperature=temperature,
        max_iterations=max_iterations,
    )

    result = await run_agent(request)

    if isinstance(result, StreamingResponse):
        raise HTTPException(status_code=500, detail="Unexpected streaming response")

    return result


@router.get("/types")
async def list_agent_types() -> dict:
    """List available agent types and their capabilities."""
    orchestrator = get_orchestrator()
    agents = orchestrator.list_agents()

    # Add detailed capabilities
    capabilities_map = {
        "rag": {
            "capabilities": ["Document retrieval", "Context-aware generation", "Answer refinement"],
            "default_iterations": 10,
        },
        "research": {
            "capabilities": ["Research planning", "Multi-step investigation", "Finding synthesis"],
            "default_iterations": 15,
        },
        "data_entry": {
            "capabilities": ["Data extraction", "Validation", "Format transformation"],
            "default_iterations": 5,
        },
        "support_triage": {
            "capabilities": ["Ticket analysis", "Priority classification", "Response generation", "Team routing"],
            "default_iterations": 6,
        },
        "report": {
            "capabilities": ["Report planning", "Data gathering", "Section generation", "Multi-format output"],
            "default_iterations": 8,
        },
    }

    enriched_agents = []
    for agent in agents:
        agent_type = agent["type"]
        caps = capabilities_map.get(agent_type, {"capabilities": [], "default_iterations": 10})
        enriched_agents.append({
            **agent,
            **caps,
        })

    return {"agents": enriched_agents}
