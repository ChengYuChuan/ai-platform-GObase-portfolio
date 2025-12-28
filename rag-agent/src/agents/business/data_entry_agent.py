"""Data Entry Automation Agent.

This agent automates data entry tasks by:
1. Extracting data from documents
2. Validating the extracted data
3. Transforming it into the required format
4. Preparing it for database entry
"""

from typing import Any, AsyncIterator

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage
from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph

from src.agents.base import AgentConfig, AgentResult, BaseAgent
from src.agents.tools.data_tools import ExtractDataTool, ValidateDataTool, TransformDataTool
from src.agents.tools.document_tools import SearchDocumentsTool
from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class DataEntryState(dict):
    """State for data entry agent."""

    pass


class DataEntryAgent(BaseAgent):
    """Agent for automating data entry from documents.

    This agent can:
    - Search and retrieve relevant documents
    - Extract structured data from unstructured text
    - Validate extracted data against rules
    - Transform data into target formats
    """

    def __init__(self, config: AgentConfig | None = None) -> None:
        """Initialize the data entry agent.

        Args:
            config: Agent configuration.
        """
        super().__init__(config)
        self.settings = get_settings()

        # Initialize LLM
        self.llm = ChatOpenAI(
            model=self.config.model_name,
            temperature=0,  # Use 0 for deterministic extraction
            openai_api_key=self.settings.openai_api_key,
        )

        # Initialize tools
        self.tools = {
            "search": SearchDocumentsTool(),
            "extract": ExtractDataTool(),
            "validate": ValidateDataTool(),
            "transform": TransformDataTool(),
        }

        # Build the graph
        self.graph = self._build_graph()

    def _build_graph(self) -> StateGraph:
        """Build the data entry workflow graph.

        Returns:
            Compiled state graph.
        """
        workflow = StateGraph(dict)

        # Add nodes
        workflow.add_node("analyze_request", self._analyze_request_node)
        workflow.add_node("search_documents", self._search_documents_node)
        workflow.add_node("extract_data", self._extract_data_node)
        workflow.add_node("validate_data", self._validate_data_node)
        workflow.add_node("transform_data", self._transform_data_node)
        workflow.add_node("finalize", self._finalize_node)

        # Set entry point
        workflow.set_entry_point("analyze_request")

        # Add edges
        workflow.add_edge("analyze_request", "search_documents")
        workflow.add_edge("search_documents", "extract_data")
        workflow.add_edge("extract_data", "validate_data")
        workflow.add_conditional_edges(
            "validate_data",
            self._validation_router,
            {
                "valid": "transform_data",
                "invalid": "extract_data",  # Retry extraction
                "max_retries": "finalize",
            },
        )
        workflow.add_edge("transform_data", "finalize")
        workflow.add_edge("finalize", END)

        return workflow.compile()

    async def _analyze_request_node(self, state: dict) -> dict:
        """Analyze the data entry request."""
        request = state.get("request", "")

        logger.debug("Analyzing data entry request", request_preview=request[:100])

        # Determine what data needs to be extracted
        prompt = ChatPromptTemplate.from_template(
            """Analyze this data entry request and determine:
1. What type of data needs to be extracted
2. What fields are required
3. What document sources to search

Request: {request}

Respond in JSON format:
{{
    "data_type": "type of data (e.g., invoice, contact, order)",
    "required_fields": ["list", "of", "fields"],
    "search_queries": ["queries to find relevant documents"],
    "validation_rules": {{}}
}}"""
        )

        result = await (prompt | self.llm).ainvoke({"request": request})

        import json
        import re

        # Parse JSON from response
        try:
            json_str = result.content.strip()
            if json_str.startswith("```"):
                json_str = re.sub(r"^```(?:json)?\n?", "", json_str)
                json_str = re.sub(r"\n?```$", "", json_str)
            analysis = json.loads(json_str)
        except json.JSONDecodeError:
            analysis = {
                "data_type": "general",
                "required_fields": [],
                "search_queries": [request],
                "validation_rules": {},
            }

        return {
            **state,
            "analysis": analysis,
            "iteration": 0,
            "status": "analyzing",
        }

    async def _search_documents_node(self, state: dict) -> dict:
        """Search for relevant documents."""
        analysis = state.get("analysis", {})
        search_queries = analysis.get("search_queries", [])

        logger.debug("Searching documents", queries=search_queries)

        all_results = []
        for query in search_queries[:3]:  # Limit to 3 queries
            result = await self.tools["search"].execute(query=query, top_k=5)
            if result.is_success:
                all_results.extend(result.data)

        # Deduplicate by content
        seen_content = set()
        unique_results = []
        for r in all_results:
            content_hash = hash(r["content"][:100])
            if content_hash not in seen_content:
                seen_content.add(content_hash)
                unique_results.append(r)

        return {
            **state,
            "documents": unique_results[:10],  # Keep top 10
            "status": "documents_found",
        }

    async def _extract_data_node(self, state: dict) -> dict:
        """Extract data from documents."""
        documents = state.get("documents", [])
        analysis = state.get("analysis", {})

        logger.debug("Extracting data", num_documents=len(documents))

        if not documents:
            return {
                **state,
                "extracted_data": {},
                "status": "no_documents",
            }

        # Combine document content
        combined_text = "\n\n---\n\n".join(
            f"[Source: {d.get('source', 'Unknown')}]\n{d['content']}"
            for d in documents
        )

        # Build extraction schema from analysis
        required_fields = analysis.get("required_fields", [])
        extraction_schema = {
            field: {"type": "string"} for field in required_fields
        } if required_fields else None

        result = await self.tools["extract"].execute(
            text=combined_text,
            extraction_schema=extraction_schema,
        )

        extracted_data = result.data if result.is_success else {}

        return {
            **state,
            "extracted_data": extracted_data,
            "extraction_sources": [d.get("source", "Unknown") for d in documents],
            "status": "data_extracted",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _validate_data_node(self, state: dict) -> dict:
        """Validate extracted data."""
        extracted_data = state.get("extracted_data", {})
        analysis = state.get("analysis", {})
        validation_rules = analysis.get("validation_rules", {})

        logger.debug("Validating data", fields=list(extracted_data.keys()))

        # Add required field rules
        for field in analysis.get("required_fields", []):
            if field not in validation_rules:
                validation_rules[field] = {"required": True}

        result = await self.tools["validate"].execute(
            data=extracted_data,
            rules=validation_rules,
        )

        validation_result = result.data if result.is_success else {"valid": False, "errors": []}

        return {
            **state,
            "validation_result": validation_result,
            "status": "validated" if validation_result.get("valid") else "validation_failed",
        }

    def _validation_router(self, state: dict) -> str:
        """Route based on validation result."""
        validation_result = state.get("validation_result", {})
        iteration = state.get("iteration", 0)

        if validation_result.get("valid"):
            return "valid"
        elif iteration >= 3:
            return "max_retries"
        else:
            return "invalid"

    async def _transform_data_node(self, state: dict) -> dict:
        """Transform data into target format."""
        extracted_data = state.get("extracted_data", {})
        analysis = state.get("analysis", {})

        logger.debug("Transforming data")

        # Apply standard transformations
        transformations = [
            {"type": "format", "field": field, "format": "trim"}
            for field in extracted_data.keys()
        ]

        result = await self.tools["transform"].execute(
            data=extracted_data,
            transformations=transformations,
        )

        transformed_data = result.data if result.is_success else extracted_data

        return {
            **state,
            "transformed_data": transformed_data,
            "status": "transformed",
        }

    async def _finalize_node(self, state: dict) -> dict:
        """Finalize the data entry result."""
        logger.debug("Finalizing data entry")

        final_data = state.get("transformed_data", state.get("extracted_data", {}))
        validation_result = state.get("validation_result", {})

        return {
            **state,
            "final_data": final_data,
            "success": validation_result.get("valid", False),
            "status": "completed",
        }

    async def run(self, request: str, **kwargs: Any) -> AgentResult:
        """Run the data entry agent.

        Args:
            request: Data entry request describing what to extract.
            **kwargs: Additional arguments.

        Returns:
            AgentResult with extracted and validated data.
        """
        logger.info("Data entry agent started", request_preview=request[:50])

        initial_state = {
            "request": request,
            "documents": [],
            "extracted_data": {},
            "validation_result": {},
            "transformed_data": {},
            "final_data": {},
            "iteration": 0,
            "status": "started",
            "success": False,
        }

        try:
            final_state = await self.graph.ainvoke(initial_state)

            return AgentResult(
                answer=str(final_state.get("final_data", {})),
                sources=[
                    {"content": src, "metadata": {}}
                    for src in final_state.get("extraction_sources", [])
                ],
                iterations=final_state.get("iteration", 0),
                success=final_state.get("success", False),
                metadata={
                    "data_type": final_state.get("analysis", {}).get("data_type"),
                    "validation_errors": final_state.get("validation_result", {}).get("errors", []),
                    "final_data": final_state.get("final_data", {}),
                },
            )

        except Exception as e:
            logger.error("Data entry agent failed", error=str(e))
            return AgentResult(
                answer="",
                sources=[],
                iterations=0,
                success=False,
                error=str(e),
            )

    async def stream(self, request: str, **kwargs: Any) -> AsyncIterator[dict[str, Any]]:
        """Stream data entry agent execution.

        Args:
            request: Data entry request.
            **kwargs: Additional arguments.

        Yields:
            Intermediate states and events.
        """
        logger.info("Data entry agent streaming", request_preview=request[:50])

        initial_state = {
            "request": request,
            "documents": [],
            "extracted_data": {},
            "validation_result": {},
            "transformed_data": {},
            "final_data": {},
            "iteration": 0,
            "status": "started",
            "success": False,
        }

        async for event in self.graph.astream(initial_state):
            for node_name, state in event.items():
                yield {
                    "event": node_name,
                    "status": state.get("status", ""),
                    "iteration": state.get("iteration", 0),
                    "has_data": bool(state.get("extracted_data")),
                    "is_valid": state.get("validation_result", {}).get("valid", False),
                }


def create_data_entry_agent(
    model_name: str | None = None,
    max_iterations: int = 5,
) -> DataEntryAgent:
    """Factory function to create a data entry agent.

    Args:
        model_name: LLM model to use.
        max_iterations: Maximum extraction retries.

    Returns:
        Configured data entry agent.
    """
    settings = get_settings()

    config = AgentConfig(
        model_name=model_name or settings.llm_model,
        temperature=0,
        max_iterations=max_iterations,
    )

    return DataEntryAgent(config)
