"""Base agent implementation and types."""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, TypedDict

from langchain_core.messages import BaseMessage


class AgentState(TypedDict, total=False):
    """State for agent graph."""

    messages: list[BaseMessage]
    context: str
    question: str
    answer: str
    sources: list[dict[str, Any]]
    next_action: str
    iteration: int
    error: str | None


class ActionType(str, Enum):
    """Types of agent actions."""

    RETRIEVE = "retrieve"
    GENERATE = "generate"
    REFINE = "refine"
    SEARCH = "search"
    DONE = "done"
    ERROR = "error"


@dataclass
class AgentConfig:
    """Configuration for agents."""

    model_name: str = "gpt-4o-mini"
    temperature: float = 0.7
    max_iterations: int = 10
    retrieval_top_k: int = 5
    verbose: bool = False


@dataclass
class AgentResult:
    """Result from agent execution."""

    answer: str
    sources: list[dict[str, Any]] = field(default_factory=list)
    iterations: int = 0
    success: bool = True
    error: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


class BaseAgent(ABC):
    """Abstract base class for agents."""

    def __init__(self, config: AgentConfig | None = None) -> None:
        """Initialize the agent.

        Args:
            config: Agent configuration.
        """
        self.config = config or AgentConfig()

    @abstractmethod
    async def run(self, question: str, **kwargs: Any) -> AgentResult:
        """Run the agent on a question.

        Args:
            question: User question.
            **kwargs: Additional arguments.

        Returns:
            Agent result.
        """
        pass

    @abstractmethod
    async def stream(self, question: str, **kwargs: Any) -> Any:
        """Stream agent execution.

        Args:
            question: User question.
            **kwargs: Additional arguments.

        Yields:
            Intermediate results.
        """
        pass
