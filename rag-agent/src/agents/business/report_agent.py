"""Report Generation Agent.

This agent generates reports by:
1. Analyzing report requirements
2. Gathering relevant data from documents
3. Structuring the information
4. Generating formatted reports
"""

from typing import Any, AsyncIterator
from enum import Enum

from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph

from src.agents.base import AgentConfig, AgentResult, BaseAgent
from src.agents.tools.document_tools import SearchDocumentsTool, SummarizeDocumentTool
from src.agents.tools.data_tools import FormatOutputTool
from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class ReportFormat(str, Enum):
    """Report output formats."""

    MARKDOWN = "markdown"
    HTML = "html"
    JSON = "json"
    TEXT = "text"


class ReportType(str, Enum):
    """Types of reports."""

    SUMMARY = "summary"
    ANALYSIS = "analysis"
    COMPARISON = "comparison"
    STATUS = "status"
    RESEARCH = "research"


class ReportGenerationAgent(BaseAgent):
    """Agent for generating structured reports from documents.

    This agent can:
    - Analyze report requirements
    - Search and gather relevant information
    - Structure data into report sections
    - Generate formatted reports in various formats
    """

    def __init__(self, config: AgentConfig | None = None) -> None:
        """Initialize the report generation agent.

        Args:
            config: Agent configuration.
        """
        super().__init__(config)
        self.settings = get_settings()

        # Initialize LLM
        self.llm = ChatOpenAI(
            model=self.config.model_name,
            temperature=0.5,  # Balanced for report writing
            openai_api_key=self.settings.openai_api_key,
        )

        # Initialize tools
        self.search_tool = SearchDocumentsTool()
        self.summarize_tool = SummarizeDocumentTool()
        self.format_tool = FormatOutputTool()

        # Build the graph
        self.graph = self._build_graph()

    def _build_graph(self) -> StateGraph:
        """Build the report generation workflow graph.

        Returns:
            Compiled state graph.
        """
        workflow = StateGraph(dict)

        # Add nodes
        workflow.add_node("analyze_requirements", self._analyze_requirements_node)
        workflow.add_node("plan_report", self._plan_report_node)
        workflow.add_node("gather_data", self._gather_data_node)
        workflow.add_node("generate_sections", self._generate_sections_node)
        workflow.add_node("compile_report", self._compile_report_node)
        workflow.add_node("format_output", self._format_output_node)

        # Set entry point
        workflow.set_entry_point("analyze_requirements")

        # Add edges
        workflow.add_edge("analyze_requirements", "plan_report")
        workflow.add_edge("plan_report", "gather_data")
        workflow.add_edge("gather_data", "generate_sections")
        workflow.add_edge("generate_sections", "compile_report")
        workflow.add_edge("compile_report", "format_output")
        workflow.add_edge("format_output", END)

        return workflow.compile()

    async def _analyze_requirements_node(self, state: dict) -> dict:
        """Analyze report requirements."""
        request = state.get("request", "")
        report_type = state.get("report_type", "summary")

        logger.debug("Analyzing report requirements", request_preview=request[:100])

        prompt = ChatPromptTemplate.from_template(
            """Analyze this report request and determine the requirements.

Report Request: {request}
Report Type: {report_type}

Provide analysis in JSON format:
{{
    "title": "suggested report title",
    "purpose": "purpose of the report",
    "audience": "intended audience",
    "key_topics": ["main topics to cover"],
    "data_sources": ["types of data needed"],
    "search_queries": ["queries to find relevant information"],
    "time_scope": "time period covered if applicable",
    "depth": "brief/standard/detailed"
}}"""
        )

        result = await (prompt | self.llm).ainvoke({
            "request": request,
            "report_type": report_type,
        })

        import json
        import re

        try:
            json_str = result.content.strip()
            if json_str.startswith("```"):
                json_str = re.sub(r"^```(?:json)?\n?", "", json_str)
                json_str = re.sub(r"\n?```$", "", json_str)
            requirements = json.loads(json_str)
        except json.JSONDecodeError:
            requirements = {
                "title": "Report",
                "purpose": request,
                "audience": "general",
                "key_topics": [],
                "data_sources": [],
                "search_queries": [request],
                "depth": "standard",
            }

        return {
            **state,
            "requirements": requirements,
            "status": "requirements_analyzed",
            "iteration": 1,
        }

    async def _plan_report_node(self, state: dict) -> dict:
        """Plan the report structure."""
        requirements = state.get("requirements", {})
        report_type = state.get("report_type", "summary")

        logger.debug("Planning report structure")

        prompt = ChatPromptTemplate.from_template(
            """Create a detailed outline for this report.

Requirements: {requirements}
Report Type: {report_type}

Create a report outline in JSON format:
{{
    "sections": [
        {{
            "title": "Section Title",
            "purpose": "what this section covers",
            "key_points": ["points to address"],
            "search_queries": ["specific queries for this section"]
        }}
    ],
    "executive_summary_needed": true/false,
    "conclusion_needed": true/false,
    "appendix_topics": ["optional appendix items"]
}}"""
        )

        result = await (prompt | self.llm).ainvoke({
            "requirements": str(requirements),
            "report_type": report_type,
        })

        import json
        import re

        try:
            json_str = result.content.strip()
            if json_str.startswith("```"):
                json_str = re.sub(r"^```(?:json)?\n?", "", json_str)
                json_str = re.sub(r"\n?```$", "", json_str)
            outline = json.loads(json_str)
        except json.JSONDecodeError:
            outline = {
                "sections": [
                    {
                        "title": "Overview",
                        "purpose": "Main findings",
                        "key_points": [],
                        "search_queries": requirements.get("search_queries", []),
                    }
                ],
                "executive_summary_needed": True,
                "conclusion_needed": True,
            }

        return {
            **state,
            "outline": outline,
            "status": "report_planned",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _gather_data_node(self, state: dict) -> dict:
        """Gather data for the report."""
        outline = state.get("outline", {})
        requirements = state.get("requirements", {})

        logger.debug("Gathering report data")

        all_data = []
        sections = outline.get("sections", [])

        for section in sections:
            section_data = {
                "title": section.get("title", ""),
                "content": [],
            }

            queries = section.get("search_queries", [])
            for query in queries[:2]:
                result = await self.search_tool.execute(query=query, top_k=5)
                if result.is_success:
                    for doc in result.data:
                        section_data["content"].append({
                            "text": doc["content"],
                            "source": doc.get("source", "Unknown"),
                            "relevance": doc.get("score", 0),
                        })

            all_data.append(section_data)

        return {
            **state,
            "gathered_data": all_data,
            "status": "data_gathered",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _generate_sections_node(self, state: dict) -> dict:
        """Generate content for each report section."""
        gathered_data = state.get("gathered_data", [])
        outline = state.get("outline", {})
        requirements = state.get("requirements", {})

        logger.debug("Generating report sections")

        sections = []
        for section_data, section_outline in zip(gathered_data, outline.get("sections", [])):
            # Combine content for this section
            context = "\n\n".join(
                f"[Source: {c['source']}]\n{c['text']}"
                for c in section_data.get("content", [])[:5]
            )

            if not context:
                context = "No specific data available for this section."

            prompt = ChatPromptTemplate.from_template(
                """Write the content for this report section.

Section Title: {title}
Section Purpose: {purpose}
Key Points to Address: {key_points}

Available Information:
{context}

Report Depth: {depth}
Audience: {audience}

Write a well-structured section that:
1. Has a clear introduction
2. Addresses the key points
3. Uses evidence from the provided information
4. Maintains a professional tone

Section Content:"""
            )

            result = await (prompt | self.llm).ainvoke({
                "title": section_outline.get("title", ""),
                "purpose": section_outline.get("purpose", ""),
                "key_points": str(section_outline.get("key_points", [])),
                "context": context,
                "depth": requirements.get("depth", "standard"),
                "audience": requirements.get("audience", "general"),
            })

            sections.append({
                "title": section_outline.get("title", ""),
                "content": result.content,
                "sources": [c["source"] for c in section_data.get("content", [])[:5]],
            })

        return {
            **state,
            "generated_sections": sections,
            "status": "sections_generated",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _compile_report_node(self, state: dict) -> dict:
        """Compile all sections into a complete report."""
        sections = state.get("generated_sections", [])
        outline = state.get("outline", {})
        requirements = state.get("requirements", {})

        logger.debug("Compiling report")

        # Generate executive summary if needed
        executive_summary = ""
        if outline.get("executive_summary_needed", True):
            all_content = "\n\n".join(s["content"] for s in sections)

            prompt = ChatPromptTemplate.from_template(
                """Write an executive summary for this report.

Report Title: {title}
Report Purpose: {purpose}

Full Report Content:
{content}

Write a concise executive summary (2-3 paragraphs) that:
1. States the main purpose
2. Highlights key findings
3. Summarizes conclusions/recommendations

Executive Summary:"""
            )

            result = await (prompt | self.llm).ainvoke({
                "title": requirements.get("title", "Report"),
                "purpose": requirements.get("purpose", ""),
                "content": all_content[:5000],
            })
            executive_summary = result.content

        # Generate conclusion if needed
        conclusion = ""
        if outline.get("conclusion_needed", True):
            prompt = ChatPromptTemplate.from_template(
                """Write a conclusion for this report.

Report Purpose: {purpose}
Key Findings from Sections:
{sections}

Write a brief conclusion that:
1. Summarizes the main findings
2. Provides recommendations if applicable
3. Suggests next steps

Conclusion:"""
            )

            section_summaries = "\n".join(
                f"- {s['title']}: {s['content'][:200]}..."
                for s in sections
            )

            result = await (prompt | self.llm).ainvoke({
                "purpose": requirements.get("purpose", ""),
                "sections": section_summaries,
            })
            conclusion = result.content

        # Compile full report
        compiled_report = {
            "title": requirements.get("title", "Report"),
            "executive_summary": executive_summary,
            "sections": sections,
            "conclusion": conclusion,
            "sources": list(set(
                source
                for s in sections
                for source in s.get("sources", [])
            )),
        }

        return {
            **state,
            "compiled_report": compiled_report,
            "status": "report_compiled",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _format_output_node(self, state: dict) -> dict:
        """Format the report for output."""
        compiled_report = state.get("compiled_report", {})
        output_format = state.get("output_format", "markdown")

        logger.debug("Formatting report output", format=output_format)

        if output_format == "markdown":
            # Format as Markdown
            lines = [
                f"# {compiled_report.get('title', 'Report')}",
                "",
            ]

            if compiled_report.get("executive_summary"):
                lines.extend([
                    "## Executive Summary",
                    "",
                    compiled_report["executive_summary"],
                    "",
                ])

            for section in compiled_report.get("sections", []):
                lines.extend([
                    f"## {section['title']}",
                    "",
                    section["content"],
                    "",
                ])

            if compiled_report.get("conclusion"):
                lines.extend([
                    "## Conclusion",
                    "",
                    compiled_report["conclusion"],
                    "",
                ])

            if compiled_report.get("sources"):
                lines.extend([
                    "## Sources",
                    "",
                ])
                for source in compiled_report["sources"]:
                    lines.append(f"- {source}")

            formatted_output = "\n".join(lines)

        elif output_format == "html":
            # Format as HTML
            html_parts = [
                f"<h1>{compiled_report.get('title', 'Report')}</h1>",
            ]

            if compiled_report.get("executive_summary"):
                html_parts.extend([
                    "<h2>Executive Summary</h2>",
                    f"<p>{compiled_report['executive_summary'].replace(chr(10), '</p><p>')}</p>",
                ])

            for section in compiled_report.get("sections", []):
                html_parts.extend([
                    f"<h2>{section['title']}</h2>",
                    f"<p>{section['content'].replace(chr(10), '</p><p>')}</p>",
                ])

            if compiled_report.get("conclusion"):
                html_parts.extend([
                    "<h2>Conclusion</h2>",
                    f"<p>{compiled_report['conclusion'].replace(chr(10), '</p><p>')}</p>",
                ])

            formatted_output = "\n".join(html_parts)

        elif output_format == "json":
            import json
            formatted_output = json.dumps(compiled_report, indent=2, ensure_ascii=False)

        else:
            # Plain text
            lines = [compiled_report.get("title", "Report").upper(), "=" * 50, ""]

            if compiled_report.get("executive_summary"):
                lines.extend(["EXECUTIVE SUMMARY", "-" * 20, compiled_report["executive_summary"], ""])

            for section in compiled_report.get("sections", []):
                lines.extend([section["title"].upper(), "-" * 20, section["content"], ""])

            if compiled_report.get("conclusion"):
                lines.extend(["CONCLUSION", "-" * 20, compiled_report["conclusion"]])

            formatted_output = "\n".join(lines)

        return {
            **state,
            "formatted_report": formatted_output,
            "success": True,
            "status": "completed",
        }

    async def run(
        self,
        request: str,
        report_type: str = "summary",
        output_format: str = "markdown",
        **kwargs: Any,
    ) -> AgentResult:
        """Run the report generation agent.

        Args:
            request: Report request/topic.
            report_type: Type of report to generate.
            output_format: Output format (markdown, html, json, text).
            **kwargs: Additional arguments.

        Returns:
            AgentResult with generated report.
        """
        logger.info("Report generation agent started", request_preview=request[:50])

        initial_state = {
            "request": request,
            "report_type": report_type,
            "output_format": output_format,
            "requirements": {},
            "outline": {},
            "gathered_data": [],
            "generated_sections": [],
            "compiled_report": {},
            "formatted_report": "",
            "iteration": 0,
            "status": "started",
            "success": False,
        }

        try:
            final_state = await self.graph.ainvoke(initial_state)

            return AgentResult(
                answer=final_state.get("formatted_report", ""),
                sources=[
                    {"content": source, "metadata": {}}
                    for source in final_state.get("compiled_report", {}).get("sources", [])
                ],
                iterations=final_state.get("iteration", 0),
                success=final_state.get("success", False),
                metadata={
                    "report_type": report_type,
                    "output_format": output_format,
                    "title": final_state.get("requirements", {}).get("title", ""),
                    "sections_count": len(final_state.get("generated_sections", [])),
                },
            )

        except Exception as e:
            logger.error("Report generation agent failed", error=str(e))
            return AgentResult(
                answer="",
                sources=[],
                iterations=0,
                success=False,
                error=str(e),
            )

    async def stream(
        self,
        request: str,
        report_type: str = "summary",
        output_format: str = "markdown",
        **kwargs: Any,
    ) -> AsyncIterator[dict[str, Any]]:
        """Stream report generation execution.

        Args:
            request: Report request/topic.
            report_type: Type of report.
            output_format: Output format.
            **kwargs: Additional arguments.

        Yields:
            Intermediate states and events.
        """
        logger.info("Report generation agent streaming", request_preview=request[:50])

        initial_state = {
            "request": request,
            "report_type": report_type,
            "output_format": output_format,
            "requirements": {},
            "outline": {},
            "gathered_data": [],
            "generated_sections": [],
            "compiled_report": {},
            "formatted_report": "",
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
                    "sections_ready": len(state.get("generated_sections", [])),
                    "title": state.get("requirements", {}).get("title", ""),
                }


def create_report_agent(
    model_name: str | None = None,
) -> ReportGenerationAgent:
    """Factory function to create a report generation agent.

    Args:
        model_name: LLM model to use.

    Returns:
        Configured report generation agent.
    """
    settings = get_settings()

    config = AgentConfig(
        model_name=model_name or settings.llm_model,
        temperature=0.5,
        max_iterations=10,
    )

    return ReportGenerationAgent(config)
