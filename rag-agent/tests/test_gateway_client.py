"""Tests for Go Gateway client module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
import json

from src.integrations.gateway_client import (
    ChatMessage,
    ChatCompletionResponse,
    EmbeddingResponse,
    GatewayClient,
    GatewayLLM,
    GatewayEmbeddings,
    get_gateway_client,
)


class TestChatMessage:
    """Tests for ChatMessage dataclass."""

    def test_create_message(self):
        """Test creating a chat message."""
        msg = ChatMessage(role="user", content="Hello")
        assert msg.role == "user"
        assert msg.content == "Hello"


class TestChatCompletionResponse:
    """Tests for ChatCompletionResponse dataclass."""

    def test_create_response(self):
        """Test creating a chat completion response."""
        response = ChatCompletionResponse(
            id="123",
            model="gpt-4o-mini",
            content="Hello back!",
            usage={"prompt_tokens": 10, "completion_tokens": 5},
            finish_reason="stop",
        )
        assert response.id == "123"
        assert response.model == "gpt-4o-mini"
        assert response.content == "Hello back!"
        assert response.usage["prompt_tokens"] == 10

    def test_create_response_minimal(self):
        """Test creating response with minimal fields."""
        response = ChatCompletionResponse(
            id="123",
            model="gpt-4",
            content="Test",
        )
        assert response.usage is None
        assert response.finish_reason is None


class TestEmbeddingResponse:
    """Tests for EmbeddingResponse dataclass."""

    def test_create_embedding_response(self):
        """Test creating an embedding response."""
        response = EmbeddingResponse(
            embedding=[0.1, 0.2, 0.3],
            model="text-embedding-3-small",
            usage={"total_tokens": 5},
        )
        assert len(response.embedding) == 3
        assert response.model == "text-embedding-3-small"


class TestGatewayClient:
    """Tests for GatewayClient class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            assert client.base_url == "http://localhost:8080"
            assert client.api_key == "test-key"
            assert client.timeout == 120.0

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        with patch("src.integrations.gateway_client.get_settings"):
            client = GatewayClient(
                base_url="http://custom:9090/",
                api_key="custom-key",
                timeout=60.0,
            )

            assert client.base_url == "http://custom:9090"  # trailing slash removed
            assert client.api_key == "custom-key"
            assert client.timeout == 60.0

    @pytest.mark.asyncio
    async def test_get_client_creates_httpx_client(self):
        """Test that _get_client creates an HTTP client."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()
            http_client = await client._get_client()

            assert http_client is not None
            assert client._client is http_client

            # Cleanup
            await client.close()

    @pytest.mark.asyncio
    async def test_close(self):
        """Test closing the client."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()
            await client._get_client()

            await client.close()

            assert client._client is None

    @pytest.mark.asyncio
    async def test_health_check_success(self):
        """Test health check success."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = None

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.status_code = 200

            mock_http_client = MagicMock()
            mock_http_client.get = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            result = await client.health_check()

            assert result is True

    @pytest.mark.asyncio
    async def test_health_check_failure(self):
        """Test health check failure."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = None

            client = GatewayClient()

            mock_http_client = AsyncMock()
            mock_http_client.get.side_effect = Exception("Connection error")
            client._client = mock_http_client

            result = await client.health_check()

            assert result is False

    @pytest.mark.asyncio
    async def test_chat_completion(self):
        """Test chat completion request."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.json.return_value = {
                "id": "chatcmpl-123",
                "model": "gpt-4o-mini",
                "choices": [
                    {
                        "message": {"content": "Hello! How can I help?"},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {"prompt_tokens": 10, "completion_tokens": 8},
            }
            mock_response.raise_for_status = MagicMock()

            mock_http_client = MagicMock()
            mock_http_client.post = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            messages = [ChatMessage(role="user", content="Hello")]
            response = await client.chat_completion(messages)

            assert isinstance(response, ChatCompletionResponse)
            assert response.id == "chatcmpl-123"
            assert response.content == "Hello! How can I help?"
            assert response.finish_reason == "stop"

    @pytest.mark.asyncio
    async def test_chat_completion_with_params(self):
        """Test chat completion with custom parameters."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.json.return_value = {
                "id": "123",
                "model": "gpt-4",
                "choices": [{"message": {"content": "Test"}, "finish_reason": "stop"}],
            }
            mock_response.raise_for_status = MagicMock()

            mock_http_client = MagicMock()
            mock_http_client.post = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            messages = [ChatMessage(role="user", content="Test")]
            await client.chat_completion(
                messages,
                model="gpt-4",
                temperature=0.5,
                max_tokens=100,
            )

            # Verify the payload
            call_args = mock_http_client.post.call_args
            payload = call_args.kwargs["json"]
            assert payload["model"] == "gpt-4"
            assert payload["temperature"] == 0.5
            assert payload["max_tokens"] == 100

    @pytest.mark.asyncio
    async def test_embedding_single_text(self):
        """Test embedding single text."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.json.return_value = {
                "model": "text-embedding-3-small",
                "data": [{"embedding": [0.1, 0.2, 0.3]}],
                "usage": {"total_tokens": 5},
            }
            mock_response.raise_for_status = MagicMock()

            mock_http_client = MagicMock()
            mock_http_client.post = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            results = await client.embedding("Hello world")

            assert len(results) == 1
            assert isinstance(results[0], EmbeddingResponse)
            assert results[0].embedding == [0.1, 0.2, 0.3]

    @pytest.mark.asyncio
    async def test_embedding_multiple_texts(self):
        """Test embedding multiple texts."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.json.return_value = {
                "model": "text-embedding-3-small",
                "data": [
                    {"embedding": [0.1, 0.2, 0.3]},
                    {"embedding": [0.4, 0.5, 0.6]},
                ],
                "usage": {"total_tokens": 10},
            }
            mock_response.raise_for_status = MagicMock()

            mock_http_client = MagicMock()
            mock_http_client.post = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            results = await client.embedding(["Text 1", "Text 2"])

            assert len(results) == 2
            assert results[0].embedding == [0.1, 0.2, 0.3]
            assert results[1].embedding == [0.4, 0.5, 0.6]

    @pytest.mark.asyncio
    async def test_list_models(self):
        """Test listing available models."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.json.return_value = {
                "data": [
                    {"id": "gpt-4o-mini", "provider": "openai"},
                    {"id": "claude-3-opus", "provider": "anthropic"},
                ]
            }
            mock_response.raise_for_status = MagicMock()

            mock_http_client = MagicMock()
            mock_http_client.get = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            models = await client.list_models()

            assert len(models) == 2
            assert models[0]["id"] == "gpt-4o-mini"

    @pytest.mark.asyncio
    async def test_get_usage(self):
        """Test getting usage statistics."""
        with patch("src.integrations.gateway_client.get_settings") as mock_settings:
            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            client = GatewayClient()

            mock_response = MagicMock()
            mock_response.json.return_value = {
                "total_requests": 100,
                "total_tokens": 50000,
            }
            mock_response.raise_for_status = MagicMock()

            mock_http_client = MagicMock()
            mock_http_client.get = AsyncMock(return_value=mock_response)
            mock_http_client.is_closed = False
            client._client = mock_http_client

            usage = await client.get_usage()

            assert usage["total_requests"] == 100
            assert usage["total_tokens"] == 50000


class TestGatewayLLM:
    """Tests for GatewayLLM class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = MagicMock()
            mock_get.return_value = mock_client

            llm = GatewayLLM()

            assert llm.model == "gpt-4o-mini"
            assert llm.temperature == 0.7
            assert llm.max_tokens is None

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = MagicMock()
            mock_get.return_value = mock_client

            llm = GatewayLLM(
                model="gpt-4",
                temperature=0.5,
                max_tokens=500,
            )

            assert llm.model == "gpt-4"
            assert llm.temperature == 0.5
            assert llm.max_tokens == 500

    @pytest.mark.asyncio
    async def test_ainvoke(self):
        """Test async invocation."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = AsyncMock()
            mock_client.chat_completion.return_value = ChatCompletionResponse(
                id="123",
                model="gpt-4o-mini",
                content="Generated response",
            )
            mock_get.return_value = mock_client

            llm = GatewayLLM()
            result = await llm.ainvoke("Test prompt")

            assert result == "Generated response"
            mock_client.chat_completion.assert_called_once()

    @pytest.mark.asyncio
    async def test_astream(self):
        """Test async streaming."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = AsyncMock()

            # Mock streaming response
            async def mock_stream(*args, **kwargs):
                yield "chunk1"
                yield "chunk2"
                yield "chunk3"

            mock_client.chat_completion.return_value = mock_stream()
            mock_get.return_value = mock_client

            llm = GatewayLLM()
            chunks = []
            async for chunk in llm.astream("Test prompt"):
                chunks.append(chunk)

            assert chunks == ["chunk1", "chunk2", "chunk3"]


class TestGatewayEmbeddings:
    """Tests for GatewayEmbeddings class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = MagicMock()
            mock_get.return_value = mock_client

            embeddings = GatewayEmbeddings()

            assert embeddings.model == "text-embedding-3-small"

    def test_init_custom_model(self):
        """Test initialization with custom model."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = MagicMock()
            mock_get.return_value = mock_client

            embeddings = GatewayEmbeddings(model="text-embedding-ada-002")

            assert embeddings.model == "text-embedding-ada-002"

    @pytest.mark.asyncio
    async def test_aembed_documents(self):
        """Test async document embedding."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = AsyncMock()
            mock_client.embedding.return_value = [
                EmbeddingResponse(embedding=[0.1, 0.2], model="test"),
                EmbeddingResponse(embedding=[0.3, 0.4], model="test"),
            ]
            mock_get.return_value = mock_client

            embeddings = GatewayEmbeddings()
            result = await embeddings.aembed_documents(["Doc 1", "Doc 2"])

            assert len(result) == 2
            assert result[0] == [0.1, 0.2]
            assert result[1] == [0.3, 0.4]

    @pytest.mark.asyncio
    async def test_aembed_query(self):
        """Test async query embedding."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = AsyncMock()
            mock_client.embedding.return_value = [
                EmbeddingResponse(embedding=[0.1, 0.2, 0.3], model="test"),
            ]
            mock_get.return_value = mock_client

            embeddings = GatewayEmbeddings()
            result = await embeddings.aembed_query("Test query")

            assert result == [0.1, 0.2, 0.3]

    @pytest.mark.asyncio
    async def test_aembed_query_empty_response(self):
        """Test query embedding with empty response."""
        with patch("src.integrations.gateway_client.get_gateway_client") as mock_get:
            mock_client = AsyncMock()
            mock_client.embedding.return_value = []
            mock_get.return_value = mock_client

            embeddings = GatewayEmbeddings()
            result = await embeddings.aembed_query("Test query")

            assert result == []


class TestGetGatewayClient:
    """Tests for get_gateway_client function."""

    def test_get_gateway_client_singleton(self):
        """Test that get_gateway_client returns singleton."""
        with patch("src.integrations.gateway_client._gateway_client", None), \
             patch("src.integrations.gateway_client.get_settings") as mock_settings:

            mock_settings.return_value.gateway_url = "http://localhost:8080"
            mock_settings.return_value.gateway_api_key = "test-key"

            # Reset global
            import src.integrations.gateway_client as gw_module
            gw_module._gateway_client = None

            client1 = get_gateway_client()
            client2 = get_gateway_client()

            assert client1 is client2
