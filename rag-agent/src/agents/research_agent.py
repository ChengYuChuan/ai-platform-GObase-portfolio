"""Research Agent using LangGraph for multi-step research tasks."""

from typing import Any, AsyncIterator, Literal
from dataclasses import dataclass

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_core.prompts import ChatPromptTemplate
from langchain_core.tools import Tool
from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph
from langgraph.prebuilt import ToolNode

from src.agents.base import AgentConfig, AgentResult, BaseAgent
from src.core import get_logger
from src.core.config import get_settings
from src.rag.retrieval.retriever import get_retriever
from src.rag.chain import format_docs

logger = get_logger(__name__)


@dataclass
class ResearchState:
    """State for research agent."""

    question: str
    plan: list[str]
    current_step: int
    findings: list[dict[str, Any]]
    synthesis: str
    iteration: int
    status: str  # "planning", "researching", "synthesizing", "done", "error"
    error: str | None = None


# Research agent prompts
PLANNER_PROMPT = """You are a research planning agent. Given a research question, create a step-by-step plan.

Research Question: {question}

Create a plan with 3-5 specific research steps. Each step should be a focused query.
Format your response as a numbered list:
1. [First research step]
2. [Second research step]
...

Plan:"""


RESEARCHER_PROMPT = """You are a research agent. Given the context from documents, extract key findings.

Research Step: {step}
Context:
{context}

Extract and summarize the key findings relevant to this research step.
Be specific and cite relevant information from the context.

Findings:"""


SYNTHESIZER_PROMPT = """You are a research synthesis agent. Combine research findings into a comprehensive answer.

Original Question: {question}

Research Findings:
{findings}

Synthesize these findings into a clear, comprehensive answer to the original question.
Organize the information logically and highlight key insights.

Synthesis:"""


class ResearchAgent(BaseAgent):
    """Research Agent for multi-step research tasks using LangGraph."""

    def __init__(self, config: AgentConfig | None = None) -> None:
        """Initialize the research agent.

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
        """Build the research workflow graph.

        Returns:
            Compiled state graph.
        """
        # Create the graph with dict state type
        workflow = StateGraph(dict)

        # Add nodes
        workflow.add_node("plan", self._plan_node)
        workflow.add_node("research", self._research_node)
        workflow.add_node("synthesize", self._synthesize_node)

        # Set entry point
        workflow.set_entry_point("plan")

        # Add edges
        workflow.add_edge("plan", "research")
        workflow.add_conditional_edges(
            "research",
            self._should_continue_research,
            {
                "continue": "research",
                "synthesize": "synthesize",
            },
        )
        workflow.add_edge("synthesize", END)

        return workflow.compile()

    async def _plan_node(self, state: dict) -> dict:
        """Create a research plan.

        Args:
            state: Current state.

        Returns:
            Updated state with plan.
        """
        question = state.get("question", "")

        logger.debug("Creating research plan", question_preview=question[:50])

        prompt = ChatPromptTemplate.from_template(PLANNER_PROMPT)
        chain = prompt | self.llm

        result = await chain.ainvoke({"question": question})

        # Parse the plan from the response
        plan_text = result.content
        plan_steps = []

        for line in plan_text.split("\n"):
            line = line.strip()
            if line and (line[0].isdigit() or line.startswith("-")):
                # Remove numbering and bullet points
                step = line.lstrip("0123456789.-) ").strip()
                if step:
                    plan_steps.append(step)

        logger.info("Research plan created", num_steps=len(plan_steps))

        return {
            **state,
            "plan": plan_steps,
            "current_step": 0,
            "findings": [],
            "status": "researching",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _research_node(self, state: dict) -> dict:
        """Execute a research step.

        Args:
            state: Current state.

        Returns:
            Updated state with findings.
        """
        plan = state.get("plan", [])
        current_step = state.get("current_step", 0)
        findings = state.get("findings", [])

        if current_step >= len(plan):
            return {**state, "status": "synthesizing"}

        step = plan[current_step]
        logger.debug("Researching step", step=step, index=current_step)

        # Retrieve documents for this step
        docs = await self.retriever._aget_relevant_documents(step)
        context = format_docs(docs)

        # Extract findings
        prompt = ChatPromptTemplate.from_template(RESEARCHER_PROMPT)
        chain = prompt | self.llm

        result = await chain.ainvoke({
            "step": step,
            "context": context,
        })

        # Add findings
        step_findings = {
            "step": step,
            "step_index": current_step,
            "findings": result.content,
            "sources": [
                {
                    "content": doc.page_content[:200] + "...",
                    "metadata": doc.metadata,
                }
                for doc in docs
            ],
        }

        return {
            **state,
            "current_step": current_step + 1,
            "findings": findings + [step_findings],
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _synthesize_node(self, state: dict) -> dict:
        """Synthesize research findings.

        Args:
            state: Current state.

        Returns:
            Updated state with synthesis.
        """
        question = state.get("question", "")
        findings = state.get("findings", [])

        logger.debug("Synthesizing findings", num_findings=len(findings))

        # Format findings for synthesis
        findings_text = ""
        for f in findings:
            findings_text += f"\n## {f['step']}\n{f['findings']}\n"

        prompt = ChatPromptTemplate.from_template(SYNTHESIZER_PROMPT)
        chain = prompt | self.llm

        result = await chain.ainvoke({
            "question": question,
            "findings": findings_text,
        })

        return {
            **state,
            "synthesis": result.content,
            "status": "done",
            "iteration": state.get("iteration", 0) + 1,
        }

    def _should_continue_research(self, state: dict) -> str:
        """Decide whether to continue researching or synthesize.

        Args:
            state: Current state.

        Returns:
            "continue" or "synthesize".
        """
        plan = state.get("plan", [])
        current_step = state.get("current_step", 0)
        iteration = state.get("iteration", 0)

        # Check if we've completed all steps or hit max iterations
        if current_step >= len(plan):
            return "synthesize"

        if iteration >= self.config.max_iterations:
            logger.warning("Max iterations reached, synthesizing")
            return "synthesize"

        return "continue"

    async def run(self, question: str, **kwargs: Any) -> AgentResult:
        """Run the research agent on a question.

        Args:
            question: Research question.
            **kwargs: Additional arguments.

        Returns:
            Agent result with synthesis and findings.
        """
        logger.info("Research agent started", question_preview=question[:50])

        initial_state = {
            "question": question,
            "plan": [],
            "current_step": 0,
            "findings": [],
            "synthesis": "",
            "iteration": 0,
            "status": "planning",
            "error": None,
        }

        try:
            # Run the graph
            final_state = await self.graph.ainvoke(initial_state)

            # Collect all sources from findings
            all_sources = []
            for finding in final_state.get("findings", []):
                all_sources.extend(finding.get("sources", []))

            logger.info(
                "Research agent completed",
                iterations=final_state.get("iteration", 0),
                num_findings=len(final_state.get("findings", [])),
            )

            return AgentResult(
                answer=final_state.get("synthesis", ""),
                sources=all_sources,
                iterations=final_state.get("iteration", 0),
                success=True,
                metadata={
                    "question": question,
                    "plan": final_state.get("plan", []),
                    "findings": final_state.get("findings", []),
                },
            )

        except Exception as e:
            logger.error("Research agent failed", error=str(e))
            return AgentResult(
                answer="",
                sources=[],
                iterations=0,
                success=False,
                error=str(e),
            )

    async def stream(self, question: str, **kwargs: Any) -> AsyncIterator[dict[str, Any]]:
        """Stream research agent execution.

        Args:
            question: Research question.
            **kwargs: Additional arguments.

        Yields:
            Intermediate states and events.
        """
        logger.info("Research agent streaming", question_preview=question[:50])

        initial_state = {
            "question": question,
            "plan": [],
            "current_step": 0,
            "findings": [],
            "synthesis": "",
            "iteration": 0,
            "status": "planning",
            "error": None,
        }

        async for event in self.graph.astream(initial_state):
            for node_name, state in event.items():
                yield {
                    "event": node_name,
                    "status": state.get("status", ""),
                    "iteration": state.get("iteration", 0),
                    "current_step": state.get("current_step", 0),
                    "total_steps": len(state.get("plan", [])),
                    "num_findings": len(state.get("findings", [])),
                }

                # Yield plan when created
                if node_name == "plan" and state.get("plan"):
                    yield {
                        "event": "plan_created",
                        "plan": state.get("plan", []),
                    }

                # Yield findings as they're collected
                if node_name == "research" and state.get("findings"):
                    latest_finding = state.get("findings", [])[-1]
                    yield {
                        "event": "finding_added",
                        "finding": latest_finding,
                    }

                # Yield synthesis when complete
                if node_name == "synthesize" and state.get("synthesis"):
                    yield {
                        "event": "synthesis_complete",
                        "synthesis": state.get("synthesis"),
                    }


def create_research_agent(
    model_name: str | None = None,
    temperature: float = 0.7,
    max_iterations: int = 15,
    retrieval_top_k: int = 5,
) -> ResearchAgent:
    """Factory function to create a research agent.

    Args:
        model_name: LLM model to use.
        temperature: Generation temperature.
        max_iterations: Maximum agent iterations.
        retrieval_top_k: Number of documents to retrieve per step.

    Returns:
        Configured research agent.
    """
    settings = get_settings()

    config = AgentConfig(
        model_name=model_name or settings.llm_model,
        temperature=temperature,
        max_iterations=max_iterations,
        retrieval_top_k=retrieval_top_k,
    )

    return ResearchAgent(config)
