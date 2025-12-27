"""Agent API endpoints for autonomous AI tasks."""

import asyncio
from typing import Any, AsyncIterator
from uuid import uuid4

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from src.core import get_logger
from src.core.config import get_settings
from src.agents.rag_agent import create_rag_agent
from src.agents.research_agent import create_research_agent

router = APIRouter()
logger = get_logger(__name__)


class AgentRequest(BaseModel):
    """Request for agent execution."""

    question: str = Field(..., min_length=1, max_length=2000)
    agent_type: str = Field(default="rag", pattern="^(rag|research)$")
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

        # Create the appropriate agent
        if request.agent_type == "research":
            agent = create_research_agent(
                model_name=request.model,
                temperature=request.temperature,
                max_iterations=request.max_iterations,
            )
        else:
            agent = create_rag_agent(
                model_name=request.model,
                temperature=request.temperature,
                max_iterations=request.max_iterations,
            )

        # Run the agent
        result = await agent.run(request.question)

        response_id = str(uuid4())

        return AgentResponse(
            id=response_id,
            agent_type=request.agent_type,
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
        # Create the appropriate agent
        if request.agent_type == "research":
            agent = create_research_agent(
                model_name=request.model,
                temperature=request.temperature,
                max_iterations=request.max_iterations,
            )
        else:
            agent = create_rag_agent(
                model_name=request.model,
                temperature=request.temperature,
                max_iterations=request.max_iterations,
            )

        # Stream agent execution
        async for event in agent.stream(request.question):
            event_data = {
                "id": response_id,
                "agent_type": request.agent_type,
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
    question: str = Field(..., min_length=1, max_length=2000),
    model: str | None = None,
    temperature: float = 0.7,
    max_iterations: int = 10,
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
    question: str = Field(..., min_length=1, max_length=2000),
    model: str | None = None,
    temperature: float = 0.7,
    max_iterations: int = 15,
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
    return {
        "agents": [
            {
                "type": "rag",
                "name": "RAG Agent",
                "description": "Retrieval-Augmented Generation agent for document Q&A. "
                "Uses a graph-based workflow to retrieve, generate, and refine answers.",
                "capabilities": [
                    "Document retrieval",
                    "Context-aware generation",
                    "Answer refinement",
                ],
                "default_iterations": 10,
            },
            {
                "type": "research",
                "name": "Research Agent",
                "description": "Multi-step research agent for complex questions. "
                "Creates a research plan and systematically investigates each aspect.",
                "capabilities": [
                    "Research planning",
                    "Multi-step investigation",
                    "Finding synthesis",
                ],
                "default_iterations": 15,
            },
        ]
    }
