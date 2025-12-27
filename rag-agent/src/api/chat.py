"""Chat API endpoints for RAG-powered conversations."""

import asyncio
from typing import Annotated, Any, AsyncIterator
from uuid import uuid4

from fastapi import APIRouter, HTTPException, Query
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from src.core import get_logger
from src.core.config import get_settings
from src.rag.chain import (
    get_rag_chain,
    get_conversational_chain,
    clear_session,
    RAGChain,
    ConversationalRAGChain,
)

router = APIRouter()
logger = get_logger(__name__)


class ChatMessage(BaseModel):
    """Chat message."""

    role: str = Field(..., pattern="^(user|assistant|system)$")
    content: str


class ChatRequest(BaseModel):
    """Request for chat completion."""

    messages: list[ChatMessage]
    session_id: str | None = None
    model: str | None = None
    temperature: float = Field(default=0.7, ge=0.0, le=2.0)
    stream: bool = False
    retriever_type: str = Field(default="semantic", pattern="^(semantic|hybrid|contextual)$")


class ChatSource(BaseModel):
    """Source document reference."""

    content: str
    metadata: dict[str, Any]


class ChatResponse(BaseModel):
    """Response for chat completion."""

    id: str
    message: ChatMessage
    sources: list[ChatSource]
    session_id: str | None = None
    usage: dict[str, int] | None = None


class SessionInfo(BaseModel):
    """Information about a chat session."""

    session_id: str
    history_length: int
    created: bool = False


@router.post("/completions", response_model=ChatResponse)
async def chat_completions(request: ChatRequest) -> ChatResponse | StreamingResponse:
    """Create a chat completion using RAG.

    This endpoint retrieves relevant documents and uses them
    to generate contextual responses.
    """
    if not request.messages:
        raise HTTPException(status_code=400, detail="Messages cannot be empty")

    # Get the last user message
    user_messages = [m for m in request.messages if m.role == "user"]
    if not user_messages:
        raise HTTPException(status_code=400, detail="At least one user message is required")

    question = user_messages[-1].content

    logger.info(
        "Chat completion request",
        session_id=request.session_id,
        stream=request.stream,
        question_preview=question[:50],
    )

    try:
        if request.stream:
            return StreamingResponse(
                _stream_chat_response(request, question),
                media_type="text/event-stream",
            )

        # Use conversational chain if session_id is provided
        if request.session_id:
            chain = get_conversational_chain(
                session_id=request.session_id,
                model_name=request.model,
                temperature=request.temperature,
            )

            # Add previous messages to history if this is a new session
            if len(chain.chat_history) == 0 and len(request.messages) > 1:
                for msg in request.messages[:-1]:
                    if msg.role == "user":
                        chain.chat_history.append(
                            __import__("langchain_core.messages", fromlist=["HumanMessage"]).HumanMessage(
                                content=msg.content
                            )
                        )
                    elif msg.role == "assistant":
                        chain.chat_history.append(
                            __import__("langchain_core.messages", fromlist=["AIMessage"]).AIMessage(
                                content=msg.content
                            )
                        )

            result = await chain.invoke(question)
        else:
            # Use simple RAG chain
            chain = get_rag_chain(
                model_name=request.model,
                temperature=request.temperature,
                retriever_type=request.retriever_type,
            )
            result = await chain.invoke(question)

        response_id = str(uuid4())

        return ChatResponse(
            id=response_id,
            message=ChatMessage(role="assistant", content=result["answer"]),
            sources=[
                ChatSource(
                    content=s["content"],
                    metadata=s["metadata"],
                )
                for s in result.get("sources", [])
            ],
            session_id=request.session_id,
        )

    except Exception as e:
        logger.error("Chat completion failed", error=str(e), exc_info=True)
        raise HTTPException(status_code=500, detail=f"Chat completion failed: {str(e)}")


async def _stream_chat_response(
    request: ChatRequest,
    question: str,
) -> AsyncIterator[str]:
    """Stream chat response as SSE events.

    Args:
        request: Chat request.
        question: User question.

    Yields:
        SSE event strings.
    """
    import json

    response_id = str(uuid4())

    try:
        if request.session_id:
            chain = get_conversational_chain(
                session_id=request.session_id,
                model_name=request.model,
                temperature=request.temperature,
            )
            stream = chain.stream(question)
        else:
            chain = get_rag_chain(
                model_name=request.model,
                temperature=request.temperature,
                retriever_type=request.retriever_type,
            )
            stream = chain.stream(question)

        # Stream tokens
        async for token in stream:
            event_data = {
                "id": response_id,
                "delta": {"content": token},
                "session_id": request.session_id,
            }
            yield f"data: {json.dumps(event_data)}\n\n"

        # Send done event
        yield "data: [DONE]\n\n"

    except Exception as e:
        logger.error("Stream failed", error=str(e))
        error_data = {"error": str(e)}
        yield f"data: {json.dumps(error_data)}\n\n"


@router.post("/sessions", response_model=SessionInfo)
async def create_session(
    model: str | None = None,
    temperature: float = 0.7,
) -> SessionInfo:
    """Create a new chat session.

    Sessions maintain conversation history for multi-turn conversations.
    """
    session_id = str(uuid4())

    # Initialize the session chain
    chain = get_conversational_chain(
        session_id=session_id,
        model_name=model,
        temperature=temperature,
    )

    logger.info("Session created", session_id=session_id)

    return SessionInfo(
        session_id=session_id,
        history_length=0,
        created=True,
    )


@router.get("/sessions/{session_id}", response_model=SessionInfo)
async def get_session(session_id: str) -> SessionInfo:
    """Get information about a chat session."""
    from src.rag.chain import _conversational_chain_cache

    if session_id not in _conversational_chain_cache:
        raise HTTPException(status_code=404, detail="Session not found")

    chain = _conversational_chain_cache[session_id]

    return SessionInfo(
        session_id=session_id,
        history_length=len(chain.chat_history),
    )


@router.delete("/sessions/{session_id}")
async def delete_session(session_id: str) -> dict:
    """Delete a chat session and its history."""
    if clear_session(session_id):
        logger.info("Session deleted", session_id=session_id)
        return {"success": True, "message": f"Session {session_id} deleted"}

    raise HTTPException(status_code=404, detail="Session not found")


@router.get("/sessions/{session_id}/history")
async def get_session_history(
    session_id: str,
    limit: Annotated[int, Query(ge=1, le=100)] = 50,
) -> dict:
    """Get the conversation history for a session."""
    from src.rag.chain import _conversational_chain_cache

    if session_id not in _conversational_chain_cache:
        raise HTTPException(status_code=404, detail="Session not found")

    chain = _conversational_chain_cache[session_id]
    history = chain.get_history()

    return {
        "session_id": session_id,
        "messages": history[-limit:],
        "total": len(history),
    }


@router.post("/sessions/{session_id}/clear")
async def clear_session_history(session_id: str) -> dict:
    """Clear the conversation history for a session without deleting it."""
    from src.rag.chain import _conversational_chain_cache

    if session_id not in _conversational_chain_cache:
        raise HTTPException(status_code=404, detail="Session not found")

    chain = _conversational_chain_cache[session_id]
    chain.clear_history()

    logger.info("Session history cleared", session_id=session_id)

    return {"success": True, "message": f"Session {session_id} history cleared"}
