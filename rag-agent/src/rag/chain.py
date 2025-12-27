"""RAG chain implementation using LangChain."""

from typing import Any, AsyncIterator

from langchain_core.documents import Document
from langchain_core.output_parsers import StrOutputParser
from langchain_core.prompts import ChatPromptTemplate, MessagesPlaceholder
from langchain_core.runnables import RunnablePassthrough, RunnableLambda
from langchain_core.messages import HumanMessage, AIMessage, BaseMessage
from langchain_openai import ChatOpenAI

from src.core import get_logger
from src.core.config import get_settings
from src.rag.retrieval.retriever import get_retriever, QdrantRetriever

logger = get_logger(__name__)


# Default RAG prompt template
RAG_PROMPT_TEMPLATE = """You are a helpful AI assistant. Use the following context to answer the user's question.
If you cannot find the answer in the context, say so honestly. Do not make up information.

Context:
{context}

Question: {question}

Answer:"""


CONVERSATIONAL_RAG_PROMPT = """You are a helpful AI assistant engaged in a conversation.
Use the provided context and conversation history to give accurate, helpful responses.
If the context doesn't contain relevant information, use your general knowledge but be clear about it.

Context from documents:
{context}

Current question: {question}"""


def format_docs(docs: list[Document]) -> str:
    """Format documents into a context string.

    Args:
        docs: List of documents to format.

    Returns:
        Formatted context string.
    """
    if not docs:
        return "No relevant context found."

    formatted_parts = []
    for i, doc in enumerate(docs, 1):
        source = doc.metadata.get("filename", "Unknown source")
        score = doc.metadata.get("relevance_score", 0)
        formatted_parts.append(
            f"[{i}] (Source: {source}, Relevance: {score:.2f})\n{doc.page_content}"
        )

    return "\n\n---\n\n".join(formatted_parts)


class RAGChain:
    """RAG chain for question answering."""

    def __init__(
        self,
        model_name: str | None = None,
        temperature: float = 0.7,
        retriever_type: str = "semantic",
        top_k: int | None = None,
    ) -> None:
        """Initialize the RAG chain.

        Args:
            model_name: LLM model to use.
            temperature: Generation temperature.
            retriever_type: Type of retriever to use.
            top_k: Number of documents to retrieve.
        """
        self.settings = get_settings()
        self.model_name = model_name or self.settings.llm_model
        self.temperature = temperature
        self.top_k = top_k or self.settings.retrieval_top_k

        # Initialize LLM
        self.llm = ChatOpenAI(
            model=self.model_name,
            temperature=self.temperature,
            openai_api_key=self.settings.openai_api_key,
        )

        # Initialize retriever
        self.retriever = get_retriever(
            retriever_type=retriever_type,
            top_k=self.top_k,
        )

        # Build the chain
        self._chain = self._build_chain()

    def _build_chain(self) -> Any:
        """Build the RAG chain."""
        prompt = ChatPromptTemplate.from_template(RAG_PROMPT_TEMPLATE)

        chain = (
            {
                "context": self.retriever | RunnableLambda(format_docs),
                "question": RunnablePassthrough(),
            }
            | prompt
            | self.llm
            | StrOutputParser()
        )

        return chain

    async def invoke(self, question: str) -> dict[str, Any]:
        """Run the RAG chain.

        Args:
            question: User question.

        Returns:
            Dict with answer and source documents.
        """
        logger.info("RAG chain invoked", question_preview=question[:50])

        # Get source documents for reference
        docs = await self.retriever._aget_relevant_documents(question)

        # Run the chain
        answer = await self._chain.ainvoke(question)

        return {
            "answer": answer,
            "sources": [
                {
                    "content": doc.page_content[:200] + "...",
                    "metadata": doc.metadata,
                }
                for doc in docs
            ],
            "question": question,
        }

    async def stream(self, question: str) -> AsyncIterator[str]:
        """Stream the RAG response.

        Args:
            question: User question.

        Yields:
            Response tokens.
        """
        logger.info("RAG chain streaming", question_preview=question[:50])

        async for token in self._chain.astream(question):
            yield token


class ConversationalRAGChain:
    """Conversational RAG chain with memory."""

    def __init__(
        self,
        model_name: str | None = None,
        temperature: float = 0.7,
        max_history: int = 10,
    ) -> None:
        """Initialize the conversational RAG chain.

        Args:
            model_name: LLM model to use.
            temperature: Generation temperature.
            max_history: Maximum conversation history to keep.
        """
        self.settings = get_settings()
        self.model_name = model_name or self.settings.llm_model
        self.temperature = temperature
        self.max_history = max_history

        # Initialize LLM
        self.llm = ChatOpenAI(
            model=self.model_name,
            temperature=self.temperature,
            openai_api_key=self.settings.openai_api_key,
        )

        # Initialize retriever
        self.retriever = get_retriever(top_k=self.settings.retrieval_top_k)

        # Conversation history
        self.chat_history: list[BaseMessage] = []

        # Build the chain
        self._chain = self._build_chain()

    def _build_chain(self) -> Any:
        """Build the conversational RAG chain."""
        prompt = ChatPromptTemplate.from_messages(
            [
                ("system", CONVERSATIONAL_RAG_PROMPT),
                MessagesPlaceholder(variable_name="chat_history"),
                ("human", "{question}"),
            ]
        )

        def get_context(input_dict: dict) -> dict:
            """Get context and pass through other inputs."""
            return {
                **input_dict,
                "context": format_docs(input_dict.get("docs", [])),
            }

        chain = (
            RunnableLambda(get_context)
            | prompt
            | self.llm
            | StrOutputParser()
        )

        return chain

    async def invoke(
        self,
        question: str,
        session_id: str | None = None,
    ) -> dict[str, Any]:
        """Run the conversational RAG chain.

        Args:
            question: User question.
            session_id: Optional session ID for history management.

        Returns:
            Dict with answer, sources, and updated history.
        """
        logger.info(
            "Conversational RAG invoked",
            question_preview=question[:50],
            history_length=len(self.chat_history),
        )

        # Retrieve relevant documents
        docs = await self.retriever._aget_relevant_documents(question)

        # Prepare input
        input_data = {
            "question": question,
            "docs": docs,
            "chat_history": self.chat_history[-self.max_history:],
        }

        # Run the chain
        answer = await self._chain.ainvoke(input_data)

        # Update history
        self.chat_history.append(HumanMessage(content=question))
        self.chat_history.append(AIMessage(content=answer))

        # Trim history if needed
        if len(self.chat_history) > self.max_history * 2:
            self.chat_history = self.chat_history[-self.max_history * 2:]

        return {
            "answer": answer,
            "sources": [
                {
                    "content": doc.page_content[:200] + "...",
                    "metadata": doc.metadata,
                }
                for doc in docs
            ],
            "question": question,
            "history_length": len(self.chat_history),
        }

    async def stream(
        self,
        question: str,
        session_id: str | None = None,
    ) -> AsyncIterator[str]:
        """Stream the conversational RAG response.

        Args:
            question: User question.
            session_id: Optional session ID.

        Yields:
            Response tokens.
        """
        # Retrieve relevant documents
        docs = await self.retriever._aget_relevant_documents(question)

        # Prepare input
        input_data = {
            "question": question,
            "docs": docs,
            "chat_history": self.chat_history[-self.max_history:],
        }

        # Collect full response for history
        full_response = ""

        async for token in self._chain.astream(input_data):
            full_response += token
            yield token

        # Update history after streaming completes
        self.chat_history.append(HumanMessage(content=question))
        self.chat_history.append(AIMessage(content=full_response))

    def clear_history(self) -> None:
        """Clear the conversation history."""
        self.chat_history = []
        logger.info("Conversation history cleared")

    def get_history(self) -> list[dict[str, str]]:
        """Get the conversation history.

        Returns:
            List of message dicts.
        """
        return [
            {
                "role": "user" if isinstance(msg, HumanMessage) else "assistant",
                "content": msg.content,
            }
            for msg in self.chat_history
        ]


# Chain instances cache
_chain_cache: dict[str, RAGChain] = {}
_conversational_chain_cache: dict[str, ConversationalRAGChain] = {}


def get_rag_chain(
    model_name: str | None = None,
    temperature: float = 0.7,
    retriever_type: str = "semantic",
) -> RAGChain:
    """Get or create a RAG chain.

    Args:
        model_name: LLM model to use.
        temperature: Generation temperature.
        retriever_type: Type of retriever.

    Returns:
        RAG chain instance.
    """
    cache_key = f"{model_name}_{temperature}_{retriever_type}"

    if cache_key not in _chain_cache:
        _chain_cache[cache_key] = RAGChain(
            model_name=model_name,
            temperature=temperature,
            retriever_type=retriever_type,
        )

    return _chain_cache[cache_key]


def get_conversational_chain(
    session_id: str,
    model_name: str | None = None,
    temperature: float = 0.7,
) -> ConversationalRAGChain:
    """Get or create a conversational RAG chain for a session.

    Args:
        session_id: Session identifier.
        model_name: LLM model to use.
        temperature: Generation temperature.

    Returns:
        Conversational RAG chain instance.
    """
    if session_id not in _conversational_chain_cache:
        _conversational_chain_cache[session_id] = ConversationalRAGChain(
            model_name=model_name,
            temperature=temperature,
        )

    return _conversational_chain_cache[session_id]


def clear_session(session_id: str) -> bool:
    """Clear a session's chain and history.

    Args:
        session_id: Session to clear.

    Returns:
        True if session existed and was cleared.
    """
    if session_id in _conversational_chain_cache:
        del _conversational_chain_cache[session_id]
        return True
    return False
