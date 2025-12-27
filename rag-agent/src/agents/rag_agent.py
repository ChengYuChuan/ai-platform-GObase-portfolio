"""RAG Agent using LangGraph for autonomous document Q&A."""

from typing import Any, AsyncIterator, Literal

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph

from src.agents.base import ActionType, AgentConfig, AgentResult, AgentState, BaseAgent
from src.core import get_logger
from src.core.config import get_settings
from src.rag.retrieval.retriever import get_retriever
from src.rag.chain import format_docs

logger = get_logger(__name__)


# Prompts for the RAG agent
ROUTER_PROMPT = """You are a routing agent that decides the next action based on the current state.

Given a question and any retrieved context, decide what to do next:
- "retrieve": If we need to search for relevant documents
- "generate": If we have enough context to answer the question
- "refine": If the current answer needs improvement
- "done": If we have a satisfactory answer

Question: {question}
Current Context: {context}
Current Answer: {answer}
Iteration: {iteration}

What should be the next action? Respond with only one word: retrieve, generate, refine, or done."""


GENERATOR_PROMPT = """You are a helpful AI assistant. Use the provided context to answer the question accurately.
If the context doesn't contain enough information, say so clearly.

Context:
{context}

Question: {question}

Provide a comprehensive and accurate answer:"""


REFINER_PROMPT = """You are an AI assistant that refines answers to make them more accurate and helpful.

Original Question: {question}
Context: {context}
Current Answer: {answer}

Please improve this answer by:
1. Fixing any inaccuracies
2. Adding relevant details from the context
3. Making it more clear and concise

Refined Answer:"""


class RAGAgent(BaseAgent):
    """RAG Agent using LangGraph for document Q&A with autonomous reasoning."""

    def __init__(self, config: AgentConfig | None = None) -> None:
        """Initialize the RAG agent.

        Args:
            config: Agent configuration.
        """
        super().__init__(config)
        self.settings = get_settings()

        # Initialize LLM
        self.llm = ChatOpenAI(
            model=self.config.model_name,
            temperature=self.config.temperature,
            openai_api_key=self.settings.openai_api_key,
        )

        # Initialize retriever
        self.retriever = get_retriever(top_k=self.config.retrieval_top_k)

        # Build the graph
        self.graph = self._build_graph()

    def _build_graph(self) -> StateGraph:
        """Build the LangGraph workflow.

        Returns:
            Compiled state graph.
        """
        # Create the graph
        workflow = StateGraph(AgentState)

        # Add nodes
        workflow.add_node("retrieve", self._retrieve_node)
        workflow.add_node("generate", self._generate_node)
        workflow.add_node("refine", self._refine_node)
        workflow.add_node("route", self._route_node)

        # Set entry point
        workflow.set_entry_point("route")

        # Add conditional edges from route
        workflow.add_conditional_edges(
            "route",
            self._get_next_action,
            {
                ActionType.RETRIEVE.value: "retrieve",
                ActionType.GENERATE.value: "generate",
                ActionType.REFINE.value: "refine",
                ActionType.DONE.value: END,
                ActionType.ERROR.value: END,
            },
        )

        # Add edges back to route after each action
        workflow.add_edge("retrieve", "route")
        workflow.add_edge("generate", "route")
        workflow.add_edge("refine", "route")

        return workflow.compile()

    async def _retrieve_node(self, state: AgentState) -> AgentState:
        """Retrieve relevant documents.

        Args:
            state: Current agent state.

        Returns:
            Updated state with context.
        """
        question = state.get("question", "")

        logger.debug("Retrieving documents", question_preview=question[:50])

        docs = await self.retriever._aget_relevant_documents(question)
        context = format_docs(docs)

        # Extract source information
        sources = [
            {
                "content": doc.page_content[:200] + "...",
                "metadata": doc.metadata,
            }
            for doc in docs
        ]

        return {
            **state,
            "context": context,
            "sources": sources,
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _generate_node(self, state: AgentState) -> AgentState:
        """Generate an answer based on context.

        Args:
            state: Current agent state.

        Returns:
            Updated state with answer.
        """
        question = state.get("question", "")
        context = state.get("context", "")

        logger.debug("Generating answer", question_preview=question[:50])

        prompt = ChatPromptTemplate.from_template(GENERATOR_PROMPT)
        chain = prompt | self.llm

        result = await chain.ainvoke({
            "question": question,
            "context": context,
        })

        return {
            **state,
            "answer": result.content,
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _refine_node(self, state: AgentState) -> AgentState:
        """Refine the current answer.

        Args:
            state: Current agent state.

        Returns:
            Updated state with refined answer.
        """
        question = state.get("question", "")
        context = state.get("context", "")
        answer = state.get("answer", "")

        logger.debug("Refining answer")

        prompt = ChatPromptTemplate.from_template(REFINER_PROMPT)
        chain = prompt | self.llm

        result = await chain.ainvoke({
            "question": question,
            "context": context,
            "answer": answer,
        })

        return {
            **state,
            "answer": result.content,
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _route_node(self, state: AgentState) -> AgentState:
        """Decide the next action.

        Args:
            state: Current agent state.

        Returns:
            Updated state with next action.
        """
        iteration = state.get("iteration", 0)

        # Check max iterations
        if iteration >= self.config.max_iterations:
            logger.warning("Max iterations reached", iteration=iteration)
            return {**state, "next_action": ActionType.DONE.value}

        question = state.get("question", "")
        context = state.get("context", "")
        answer = state.get("answer", "")

        # Simple routing logic
        if not context:
            return {**state, "next_action": ActionType.RETRIEVE.value}

        if not answer:
            return {**state, "next_action": ActionType.GENERATE.value}

        # Use LLM to decide if we should refine or are done
        prompt = ChatPromptTemplate.from_template(ROUTER_PROMPT)
        chain = prompt | self.llm

        result = await chain.ainvoke({
            "question": question,
            "context": context[:500],  # Truncate for routing
            "answer": answer[:500],
            "iteration": iteration,
        })

        action = result.content.strip().lower()

        # Validate action
        valid_actions = {a.value for a in ActionType}
        if action not in valid_actions:
            action = ActionType.DONE.value

        logger.debug("Route decision", action=action, iteration=iteration)

        return {**state, "next_action": action}

    def _get_next_action(self, state: AgentState) -> str:
        """Get the next action from state.

        Args:
            state: Current agent state.

        Returns:
            Next action string.
        """
        return state.get("next_action", ActionType.DONE.value)

    async def run(self, question: str, **kwargs: Any) -> AgentResult:
        """Run the RAG agent on a question.

        Args:
            question: User question.
            **kwargs: Additional arguments.

        Returns:
            Agent result with answer and sources.
        """
        logger.info("RAG agent started", question_preview=question[:50])

        initial_state: AgentState = {
            "messages": [HumanMessage(content=question)],
            "question": question,
            "context": "",
            "answer": "",
            "sources": [],
            "next_action": "",
            "iteration": 0,
            "error": None,
        }

        try:
            # Run the graph
            final_state = await self.graph.ainvoke(initial_state)

            logger.info(
                "RAG agent completed",
                iterations=final_state.get("iteration", 0),
                has_answer=bool(final_state.get("answer")),
            )

            return AgentResult(
                answer=final_state.get("answer", ""),
                sources=final_state.get("sources", []),
                iterations=final_state.get("iteration", 0),
                success=True,
                metadata={"question": question},
            )

        except Exception as e:
            logger.error("RAG agent failed", error=str(e))
            return AgentResult(
                answer="",
                sources=[],
                iterations=0,
                success=False,
                error=str(e),
            )

    async def stream(self, question: str, **kwargs: Any) -> AsyncIterator[dict[str, Any]]:
        """Stream RAG agent execution.

        Args:
            question: User question.
            **kwargs: Additional arguments.

        Yields:
            Intermediate states and events.
        """
        logger.info("RAG agent streaming", question_preview=question[:50])

        initial_state: AgentState = {
            "messages": [HumanMessage(content=question)],
            "question": question,
            "context": "",
            "answer": "",
            "sources": [],
            "next_action": "",
            "iteration": 0,
            "error": None,
        }

        async for event in self.graph.astream(initial_state):
            # Extract the node name and state
            for node_name, state in event.items():
                yield {
                    "event": node_name,
                    "iteration": state.get("iteration", 0),
                    "has_context": bool(state.get("context")),
                    "has_answer": bool(state.get("answer")),
                    "next_action": state.get("next_action", ""),
                }

                # If we have an answer, also yield it
                if state.get("answer"):
                    yield {
                        "event": "answer_update",
                        "answer": state.get("answer"),
                        "sources": state.get("sources", []),
                    }


def create_rag_agent(
    model_name: str | None = None,
    temperature: float = 0.7,
    max_iterations: int = 10,
    retrieval_top_k: int = 5,
) -> RAGAgent:
    """Factory function to create a RAG agent.

    Args:
        model_name: LLM model to use.
        temperature: Generation temperature.
        max_iterations: Maximum agent iterations.
        retrieval_top_k: Number of documents to retrieve.

    Returns:
        Configured RAG agent.
    """
    settings = get_settings()

    config = AgentConfig(
        model_name=model_name or settings.llm_model,
        temperature=temperature,
        max_iterations=max_iterations,
        retrieval_top_k=retrieval_top_k,
    )

    return RAGAgent(config)
