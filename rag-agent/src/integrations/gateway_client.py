"""Client for integrating with the Go LLM Gateway."""

import asyncio
from typing import Any, AsyncIterator
from dataclasses import dataclass

import httpx

from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


@dataclass
class ChatMessage:
    """Chat message structure."""

    role: str  # "user", "assistant", "system"
    content: str


@dataclass
class ChatCompletionResponse:
    """Response from chat completion."""

    id: str
    model: str
    content: str
    usage: dict[str, int] | None = None
    finish_reason: str | None = None


@dataclass
class EmbeddingResponse:
    """Response from embedding request."""

    embedding: list[float]
    model: str
    usage: dict[str, int] | None = None


class GatewayClient:
    """Client for communicating with the Go LLM Gateway.

    This client provides a unified interface to access LLM providers
    through the Go gateway, which handles:
    - Provider routing
    - Load balancing
    - Caching
    - Rate limiting
    - Fallback handling
    """

    def __init__(
        self,
        base_url: str | None = None,
        api_key: str | None = None,
        timeout: float = 120.0,
    ) -> None:
        """Initialize the gateway client.

        Args:
            base_url: Gateway base URL. Defaults to settings.
            api_key: API key for gateway. Defaults to settings.
            timeout: Request timeout in seconds.
        """
        settings = get_settings()
        self.base_url = (base_url or settings.gateway_url).rstrip("/")
        self.api_key = api_key or settings.gateway_api_key
        self.timeout = timeout

        self._client: httpx.AsyncClient | None = None

    async def _get_client(self) -> httpx.AsyncClient:
        """Get or create HTTP client."""
        if self._client is None or self._client.is_closed:
            headers = {"Content-Type": "application/json"}
            if self.api_key:
                headers["Authorization"] = f"Bearer {self.api_key}"

            self._client = httpx.AsyncClient(
                base_url=self.base_url,
                headers=headers,
                timeout=httpx.Timeout(self.timeout),
            )
        return self._client

    async def close(self) -> None:
        """Close the HTTP client."""
        if self._client:
            await self._client.aclose()
            self._client = None

    async def health_check(self) -> bool:
        """Check if the gateway is healthy.

        Returns:
            True if gateway is healthy.
        """
        try:
            client = await self._get_client()
            response = await client.get("/health")
            return response.status_code == 200
        except Exception as e:
            logger.warning("Gateway health check failed", error=str(e))
            return False

    async def chat_completion(
        self,
        messages: list[ChatMessage],
        model: str = "gpt-4o-mini",
        temperature: float = 0.7,
        max_tokens: int | None = None,
        stream: bool = False,
        **kwargs: Any,
    ) -> ChatCompletionResponse | AsyncIterator[str]:
        """Request chat completion from the gateway.

        Args:
            messages: List of chat messages.
            model: Model to use.
            temperature: Generation temperature.
            max_tokens: Maximum tokens to generate.
            stream: Whether to stream the response.
            **kwargs: Additional parameters.

        Returns:
            ChatCompletionResponse or async iterator of tokens.
        """
        client = await self._get_client()

        payload = {
            "model": model,
            "messages": [{"role": m.role, "content": m.content} for m in messages],
            "temperature": temperature,
            "stream": stream,
            **kwargs,
        }

        if max_tokens:
            payload["max_tokens"] = max_tokens

        logger.debug(
            "Sending chat completion request",
            model=model,
            messages_count=len(messages),
            stream=stream,
        )

        if stream:
            return self._stream_chat_completion(client, payload)

        response = await client.post("/api/v1/chat/completions", json=payload)
        response.raise_for_status()

        data = response.json()

        return ChatCompletionResponse(
            id=data.get("id", ""),
            model=data.get("model", model),
            content=data.get("choices", [{}])[0].get("message", {}).get("content", ""),
            usage=data.get("usage"),
            finish_reason=data.get("choices", [{}])[0].get("finish_reason"),
        )

    async def _stream_chat_completion(
        self,
        client: httpx.AsyncClient,
        payload: dict[str, Any],
    ) -> AsyncIterator[str]:
        """Stream chat completion response.

        Args:
            client: HTTP client.
            payload: Request payload.

        Yields:
            Response tokens.
        """
        async with client.stream(
            "POST",
            "/api/v1/chat/completions",
            json=payload,
        ) as response:
            response.raise_for_status()

            async for line in response.aiter_lines():
                if line.startswith("data: "):
                    data = line[6:]
                    if data == "[DONE]":
                        break

                    import json

                    try:
                        chunk = json.loads(data)
                        delta = chunk.get("choices", [{}])[0].get("delta", {})
                        content = delta.get("content", "")
                        if content:
                            yield content
                    except json.JSONDecodeError:
                        continue

    async def embedding(
        self,
        text: str | list[str],
        model: str = "text-embedding-3-small",
    ) -> list[EmbeddingResponse]:
        """Request embeddings from the gateway.

        Args:
            text: Text or list of texts to embed.
            model: Embedding model to use.

        Returns:
            List of embedding responses.
        """
        client = await self._get_client()

        if isinstance(text, str):
            text = [text]

        payload = {
            "model": model,
            "input": text,
        }

        logger.debug(
            "Sending embedding request",
            model=model,
            input_count=len(text),
        )

        response = await client.post("/api/v1/embeddings", json=payload)
        response.raise_for_status()

        data = response.json()

        return [
            EmbeddingResponse(
                embedding=item.get("embedding", []),
                model=data.get("model", model),
                usage=data.get("usage"),
            )
            for item in data.get("data", [])
        ]

    async def list_models(self) -> list[dict[str, Any]]:
        """List available models from the gateway.

        Returns:
            List of model information.
        """
        client = await self._get_client()
        response = await client.get("/api/v1/models")
        response.raise_for_status()

        data = response.json()
        return data.get("data", [])

    async def get_usage(self) -> dict[str, Any]:
        """Get usage statistics from the gateway.

        Returns:
            Usage statistics.
        """
        client = await self._get_client()
        response = await client.get("/api/v1/usage")
        response.raise_for_status()

        return response.json()


# Global gateway client instance
_gateway_client: GatewayClient | None = None


def get_gateway_client() -> GatewayClient:
    """Get the global gateway client instance.

    Returns:
        GatewayClient singleton.
    """
    global _gateway_client
    if _gateway_client is None:
        _gateway_client = GatewayClient()
    return _gateway_client


class GatewayLLM:
    """LangChain-compatible LLM using the Gateway.

    This class provides a LangChain-compatible interface
    for using the Go Gateway as an LLM provider.
    """

    def __init__(
        self,
        model: str = "gpt-4o-mini",
        temperature: float = 0.7,
        max_tokens: int | None = None,
    ) -> None:
        """Initialize the Gateway LLM.

        Args:
            model: Model to use.
            temperature: Generation temperature.
            max_tokens: Maximum tokens to generate.
        """
        self.model = model
        self.temperature = temperature
        self.max_tokens = max_tokens
        self._client = get_gateway_client()

    async def ainvoke(self, prompt: str) -> str:
        """Invoke the LLM asynchronously.

        Args:
            prompt: Input prompt.

        Returns:
            Generated text.
        """
        messages = [ChatMessage(role="user", content=prompt)]

        response = await self._client.chat_completion(
            messages=messages,
            model=self.model,
            temperature=self.temperature,
            max_tokens=self.max_tokens,
        )

        if isinstance(response, ChatCompletionResponse):
            return response.content
        else:
            # Stream response
            chunks = []
            async for chunk in response:
                chunks.append(chunk)
            return "".join(chunks)

    def invoke(self, prompt: str) -> str:
        """Invoke the LLM synchronously.

        Args:
            prompt: Input prompt.

        Returns:
            Generated text.
        """
        return asyncio.run(self.ainvoke(prompt))

    async def astream(self, prompt: str) -> AsyncIterator[str]:
        """Stream LLM response.

        Args:
            prompt: Input prompt.

        Yields:
            Response tokens.
        """
        messages = [ChatMessage(role="user", content=prompt)]

        response = await self._client.chat_completion(
            messages=messages,
            model=self.model,
            temperature=self.temperature,
            max_tokens=self.max_tokens,
            stream=True,
        )

        if isinstance(response, ChatCompletionResponse):
            yield response.content
        else:
            async for chunk in response:
                yield chunk


class GatewayEmbeddings:
    """LangChain-compatible embeddings using the Gateway."""

    def __init__(self, model: str = "text-embedding-3-small") -> None:
        """Initialize Gateway embeddings.

        Args:
            model: Embedding model to use.
        """
        self.model = model
        self._client = get_gateway_client()

    async def aembed_documents(self, texts: list[str]) -> list[list[float]]:
        """Embed multiple documents.

        Args:
            texts: List of texts to embed.

        Returns:
            List of embeddings.
        """
        responses = await self._client.embedding(texts, model=self.model)
        return [r.embedding for r in responses]

    async def aembed_query(self, text: str) -> list[float]:
        """Embed a single query.

        Args:
            text: Text to embed.

        Returns:
            Embedding vector.
        """
        responses = await self._client.embedding(text, model=self.model)
        return responses[0].embedding if responses else []

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        """Embed multiple documents synchronously."""
        return asyncio.run(self.aembed_documents(texts))

    def embed_query(self, text: str) -> list[float]:
        """Embed a single query synchronously."""
        return asyncio.run(self.aembed_query(text))
