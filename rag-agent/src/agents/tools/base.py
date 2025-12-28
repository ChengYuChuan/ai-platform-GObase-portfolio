"""Base tool definitions for agents."""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable, TypeVar

from pydantic import BaseModel, Field

from src.core import get_logger

logger = get_logger(__name__)

T = TypeVar("T")


class ToolStatus(str, Enum):
    """Status of tool execution."""

    SUCCESS = "success"
    ERROR = "error"
    PENDING = "pending"


@dataclass
class ToolResult:
    """Result from tool execution."""

    status: ToolStatus
    data: Any = None
    error: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    @property
    def is_success(self) -> bool:
        """Check if tool execution was successful."""
        return self.status == ToolStatus.SUCCESS

    @classmethod
    def success(cls, data: Any, **metadata: Any) -> "ToolResult":
        """Create a successful result."""
        return cls(status=ToolStatus.SUCCESS, data=data, metadata=metadata)

    @classmethod
    def error(cls, error: str, **metadata: Any) -> "ToolResult":
        """Create an error result."""
        return cls(status=ToolStatus.ERROR, error=error, metadata=metadata)


class ToolInput(BaseModel):
    """Base class for tool inputs."""

    class Config:
        extra = "forbid"


class BaseTool(ABC):
    """Abstract base class for agent tools."""

    name: str
    description: str

    def __init__(self) -> None:
        """Initialize the tool."""
        if not hasattr(self, "name"):
            self.name = self.__class__.__name__
        if not hasattr(self, "description"):
            self.description = self.__doc__ or "No description provided"

    @abstractmethod
    async def execute(self, **kwargs: Any) -> ToolResult:
        """Execute the tool with given parameters.

        Args:
            **kwargs: Tool-specific parameters.

        Returns:
            ToolResult with execution outcome.
        """
        pass

    def get_schema(self) -> dict[str, Any]:
        """Get the tool's input schema.

        Returns:
            JSON schema for tool inputs.
        """
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {},
        }

    async def __call__(self, **kwargs: Any) -> ToolResult:
        """Execute the tool (callable interface)."""
        logger.debug(f"Executing tool: {self.name}", params=kwargs)
        try:
            result = await self.execute(**kwargs)
            logger.debug(
                f"Tool {self.name} completed",
                status=result.status.value,
            )
            return result
        except Exception as e:
            logger.error(f"Tool {self.name} failed", error=str(e))
            return ToolResult.error(str(e))


def tool(
    name: str | None = None,
    description: str | None = None,
) -> Callable[[Callable[..., Any]], "FunctionTool"]:
    """Decorator to create a tool from a function.

    Args:
        name: Tool name (defaults to function name).
        description: Tool description (defaults to function docstring).

    Returns:
        Decorator that wraps function as a tool.
    """

    def decorator(func: Callable[..., Any]) -> "FunctionTool":
        tool_name = name or func.__name__
        tool_desc = description or func.__doc__ or "No description"
        return FunctionTool(func, tool_name, tool_desc)

    return decorator


class FunctionTool(BaseTool):
    """Tool wrapper for regular functions."""

    def __init__(
        self,
        func: Callable[..., Any],
        name: str,
        description: str,
    ) -> None:
        """Initialize function tool.

        Args:
            func: The function to wrap.
            name: Tool name.
            description: Tool description.
        """
        self.func = func
        self.name = name
        self.description = description
        super().__init__()

    async def execute(self, **kwargs: Any) -> ToolResult:
        """Execute the wrapped function.

        Args:
            **kwargs: Function parameters.

        Returns:
            ToolResult with function output.
        """
        import asyncio
        import inspect

        try:
            if asyncio.iscoroutinefunction(self.func):
                result = await self.func(**kwargs)
            else:
                result = self.func(**kwargs)

            return ToolResult.success(result)

        except Exception as e:
            return ToolResult.error(str(e))


class ToolRegistry:
    """Registry for managing available tools."""

    def __init__(self) -> None:
        """Initialize the tool registry."""
        self._tools: dict[str, BaseTool] = {}

    def register(self, tool: BaseTool) -> None:
        """Register a tool.

        Args:
            tool: Tool to register.
        """
        self._tools[tool.name] = tool
        logger.debug(f"Registered tool: {tool.name}")

    def unregister(self, name: str) -> bool:
        """Unregister a tool.

        Args:
            name: Tool name to unregister.

        Returns:
            True if tool was unregistered.
        """
        if name in self._tools:
            del self._tools[name]
            return True
        return False

    def get(self, name: str) -> BaseTool | None:
        """Get a tool by name.

        Args:
            name: Tool name.

        Returns:
            Tool instance or None.
        """
        return self._tools.get(name)

    def list_tools(self) -> list[dict[str, Any]]:
        """List all registered tools.

        Returns:
            List of tool schemas.
        """
        return [tool.get_schema() for tool in self._tools.values()]

    def get_all(self) -> list[BaseTool]:
        """Get all registered tools.

        Returns:
            List of all tools.
        """
        return list(self._tools.values())


# Global tool registry
_tool_registry = ToolRegistry()


def get_tool_registry() -> ToolRegistry:
    """Get the global tool registry.

    Returns:
        The tool registry singleton.
    """
    return _tool_registry
