"""Support Triage Agent.

This agent handles customer support ticket triage by:
1. Analyzing the support request
2. Classifying the issue type and priority
3. Finding relevant knowledge base articles
4. Generating a suggested response
5. Routing to the appropriate team
"""

from typing import Any, AsyncIterator
from enum import Enum

from langchain_core.prompts import ChatPromptTemplate
from langchain_openai import ChatOpenAI
from langgraph.graph import END, StateGraph

from src.agents.base import AgentConfig, AgentResult, BaseAgent
from src.agents.tools.document_tools import SearchDocumentsTool
from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class Priority(str, Enum):
    """Support ticket priority levels."""

    CRITICAL = "critical"
    HIGH = "high"
    MEDIUM = "medium"
    LOW = "low"


class IssueCategory(str, Enum):
    """Support issue categories."""

    TECHNICAL = "technical"
    BILLING = "billing"
    ACCOUNT = "account"
    FEATURE_REQUEST = "feature_request"
    BUG_REPORT = "bug_report"
    GENERAL = "general"


class SupportTriageAgent(BaseAgent):
    """Agent for triaging customer support tickets.

    This agent can:
    - Analyze support requests for intent and urgency
    - Classify issues by type and priority
    - Search knowledge base for relevant solutions
    - Generate suggested responses
    - Route tickets to appropriate teams
    """

    def __init__(self, config: AgentConfig | None = None) -> None:
        """Initialize the support triage agent.

        Args:
            config: Agent configuration.
        """
        super().__init__(config)
        self.settings = get_settings()

        # Initialize LLM
        self.llm = ChatOpenAI(
            model=self.config.model_name,
            temperature=0.3,  # Slight creativity for responses
            openai_api_key=self.settings.openai_api_key,
        )

        # Initialize tools
        self.search_tool = SearchDocumentsTool()

        # Build the graph
        self.graph = self._build_graph()

    def _build_graph(self) -> StateGraph:
        """Build the support triage workflow graph.

        Returns:
            Compiled state graph.
        """
        workflow = StateGraph(dict)

        # Add nodes
        workflow.add_node("analyze_ticket", self._analyze_ticket_node)
        workflow.add_node("classify_issue", self._classify_issue_node)
        workflow.add_node("search_knowledge_base", self._search_kb_node)
        workflow.add_node("generate_response", self._generate_response_node)
        workflow.add_node("determine_routing", self._determine_routing_node)
        workflow.add_node("finalize", self._finalize_node)

        # Set entry point
        workflow.set_entry_point("analyze_ticket")

        # Add edges
        workflow.add_edge("analyze_ticket", "classify_issue")
        workflow.add_edge("classify_issue", "search_knowledge_base")
        workflow.add_edge("search_knowledge_base", "generate_response")
        workflow.add_edge("generate_response", "determine_routing")
        workflow.add_edge("determine_routing", "finalize")
        workflow.add_edge("finalize", END)

        return workflow.compile()

    async def _analyze_ticket_node(self, state: dict) -> dict:
        """Analyze the support ticket content."""
        ticket = state.get("ticket", "")
        customer_info = state.get("customer_info", {})

        logger.debug("Analyzing support ticket", ticket_preview=ticket[:100])

        prompt = ChatPromptTemplate.from_template(
            """Analyze this customer support ticket and extract key information.

Customer Info: {customer_info}

Ticket Content:
{ticket}

Provide analysis in JSON format:
{{
    "summary": "brief summary of the issue",
    "customer_sentiment": "positive/neutral/negative/frustrated",
    "urgency_indicators": ["list of phrases indicating urgency"],
    "key_issues": ["list of main issues mentioned"],
    "customer_expectations": "what the customer expects",
    "language": "detected language"
}}"""
        )

        result = await (prompt | self.llm).ainvoke({
            "ticket": ticket,
            "customer_info": str(customer_info),
        })

        import json
        import re

        try:
            json_str = result.content.strip()
            if json_str.startswith("```"):
                json_str = re.sub(r"^```(?:json)?\n?", "", json_str)
                json_str = re.sub(r"\n?```$", "", json_str)
            analysis = json.loads(json_str)
        except json.JSONDecodeError:
            analysis = {
                "summary": ticket[:200],
                "customer_sentiment": "neutral",
                "urgency_indicators": [],
                "key_issues": [],
                "customer_expectations": "resolution",
                "language": "en",
            }

        return {
            **state,
            "analysis": analysis,
            "status": "analyzed",
            "iteration": 1,
        }

    async def _classify_issue_node(self, state: dict) -> dict:
        """Classify the issue type and priority."""
        analysis = state.get("analysis", {})
        ticket = state.get("ticket", "")

        logger.debug("Classifying issue")

        prompt = ChatPromptTemplate.from_template(
            """Based on this support ticket analysis, classify the issue.

Analysis: {analysis}

Original Ticket:
{ticket}

Classify in JSON format:
{{
    "category": "technical|billing|account|feature_request|bug_report|general",
    "priority": "critical|high|medium|low",
    "priority_reason": "reason for priority level",
    "subcategory": "more specific category",
    "tags": ["relevant", "tags"],
    "requires_escalation": true/false,
    "escalation_reason": "reason if escalation needed"
}}

Priority Guidelines:
- critical: System down, data loss, security breach
- high: Major functionality broken, revenue impact
- medium: Feature not working, workaround available
- low: Questions, minor issues, feature requests"""
        )

        result = await (prompt | self.llm).ainvoke({
            "analysis": str(analysis),
            "ticket": ticket,
        })

        import json
        import re

        try:
            json_str = result.content.strip()
            if json_str.startswith("```"):
                json_str = re.sub(r"^```(?:json)?\n?", "", json_str)
                json_str = re.sub(r"\n?```$", "", json_str)
            classification = json.loads(json_str)
        except json.JSONDecodeError:
            classification = {
                "category": "general",
                "priority": "medium",
                "priority_reason": "Unable to determine",
                "subcategory": "general",
                "tags": [],
                "requires_escalation": False,
            }

        return {
            **state,
            "classification": classification,
            "status": "classified",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _search_kb_node(self, state: dict) -> dict:
        """Search knowledge base for relevant articles."""
        analysis = state.get("analysis", {})
        classification = state.get("classification", {})

        logger.debug("Searching knowledge base")

        # Build search queries from analysis
        key_issues = analysis.get("key_issues", [])
        category = classification.get("category", "")
        summary = analysis.get("summary", "")

        search_queries = []
        if summary:
            search_queries.append(summary)
        search_queries.extend(key_issues[:2])
        if category:
            search_queries.append(f"{category} help guide")

        all_articles = []
        for query in search_queries[:3]:
            result = await self.search_tool.execute(query=query, top_k=3)
            if result.is_success:
                all_articles.extend(result.data)

        # Deduplicate
        seen = set()
        unique_articles = []
        for article in all_articles:
            content_key = article["content"][:100]
            if content_key not in seen:
                seen.add(content_key)
                unique_articles.append(article)

        return {
            **state,
            "kb_articles": unique_articles[:5],
            "status": "kb_searched",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _generate_response_node(self, state: dict) -> dict:
        """Generate a suggested response for the ticket."""
        analysis = state.get("analysis", {})
        classification = state.get("classification", {})
        kb_articles = state.get("kb_articles", [])
        ticket = state.get("ticket", "")

        logger.debug("Generating response suggestion")

        # Format KB articles for context
        kb_context = ""
        if kb_articles:
            kb_context = "\n\n".join(
                f"[Article: {a.get('source', 'KB')}]\n{a['content']}"
                for a in kb_articles[:3]
            )
        else:
            kb_context = "No relevant articles found."

        prompt = ChatPromptTemplate.from_template(
            """Generate a professional support response for this ticket.

Customer Ticket:
{ticket}

Issue Analysis:
- Summary: {summary}
- Category: {category}
- Priority: {priority}
- Customer Sentiment: {sentiment}

Relevant Knowledge Base Articles:
{kb_context}

Generate a response that:
1. Acknowledges the customer's issue
2. Shows empathy if they're frustrated
3. Provides a clear solution or next steps
4. References relevant KB articles if helpful
5. Maintains a professional but friendly tone

Response:"""
        )

        result = await (prompt | self.llm).ainvoke({
            "ticket": ticket,
            "summary": analysis.get("summary", ""),
            "category": classification.get("category", "general"),
            "priority": classification.get("priority", "medium"),
            "sentiment": analysis.get("customer_sentiment", "neutral"),
            "kb_context": kb_context,
        })

        return {
            **state,
            "suggested_response": result.content,
            "status": "response_generated",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _determine_routing_node(self, state: dict) -> dict:
        """Determine which team should handle the ticket."""
        classification = state.get("classification", {})

        logger.debug("Determining ticket routing")

        category = classification.get("category", "general")
        priority = classification.get("priority", "medium")
        requires_escalation = classification.get("requires_escalation", False)

        # Routing rules
        routing_map = {
            "technical": "engineering_support",
            "billing": "billing_team",
            "account": "account_management",
            "feature_request": "product_team",
            "bug_report": "engineering_qa",
            "general": "general_support",
        }

        assigned_team = routing_map.get(category, "general_support")

        # Escalation handling
        if requires_escalation or priority == "critical":
            assigned_team = f"{assigned_team}_escalation"
            sla_hours = 1
        elif priority == "high":
            sla_hours = 4
        elif priority == "medium":
            sla_hours = 24
        else:
            sla_hours = 72

        routing = {
            "assigned_team": assigned_team,
            "priority": priority,
            "sla_hours": sla_hours,
            "requires_escalation": requires_escalation,
            "escalation_reason": classification.get("escalation_reason"),
            "tags": classification.get("tags", []),
        }

        return {
            **state,
            "routing": routing,
            "status": "routed",
            "iteration": state.get("iteration", 0) + 1,
        }

    async def _finalize_node(self, state: dict) -> dict:
        """Finalize the triage result."""
        logger.debug("Finalizing triage")

        return {
            **state,
            "success": True,
            "status": "completed",
        }

    async def run(self, ticket: str, customer_info: dict | None = None, **kwargs: Any) -> AgentResult:
        """Run the support triage agent.

        Args:
            ticket: Support ticket content.
            customer_info: Optional customer information.
            **kwargs: Additional arguments.

        Returns:
            AgentResult with triage results.
        """
        logger.info("Support triage agent started", ticket_preview=ticket[:50])

        initial_state = {
            "ticket": ticket,
            "customer_info": customer_info or {},
            "analysis": {},
            "classification": {},
            "kb_articles": [],
            "suggested_response": "",
            "routing": {},
            "iteration": 0,
            "status": "started",
            "success": False,
        }

        try:
            final_state = await self.graph.ainvoke(initial_state)

            # Build triage summary
            triage_summary = {
                "classification": final_state.get("classification", {}),
                "routing": final_state.get("routing", {}),
                "suggested_response": final_state.get("suggested_response", ""),
                "analysis": final_state.get("analysis", {}),
            }

            return AgentResult(
                answer=final_state.get("suggested_response", ""),
                sources=[
                    {"content": a["content"][:200], "metadata": {"source": a.get("source", "KB")}}
                    for a in final_state.get("kb_articles", [])
                ],
                iterations=final_state.get("iteration", 0),
                success=final_state.get("success", False),
                metadata=triage_summary,
            )

        except Exception as e:
            logger.error("Support triage agent failed", error=str(e))
            return AgentResult(
                answer="",
                sources=[],
                iterations=0,
                success=False,
                error=str(e),
            )

    async def stream(self, ticket: str, customer_info: dict | None = None, **kwargs: Any) -> AsyncIterator[dict[str, Any]]:
        """Stream support triage execution.

        Args:
            ticket: Support ticket content.
            customer_info: Optional customer information.
            **kwargs: Additional arguments.

        Yields:
            Intermediate states and events.
        """
        logger.info("Support triage agent streaming", ticket_preview=ticket[:50])

        initial_state = {
            "ticket": ticket,
            "customer_info": customer_info or {},
            "analysis": {},
            "classification": {},
            "kb_articles": [],
            "suggested_response": "",
            "routing": {},
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
                    "priority": state.get("classification", {}).get("priority"),
                    "category": state.get("classification", {}).get("category"),
                    "assigned_team": state.get("routing", {}).get("assigned_team"),
                }


def create_support_triage_agent(
    model_name: str | None = None,
) -> SupportTriageAgent:
    """Factory function to create a support triage agent.

    Args:
        model_name: LLM model to use.

    Returns:
        Configured support triage agent.
    """
    settings = get_settings()

    config = AgentConfig(
        model_name=model_name or settings.llm_model,
        temperature=0.3,
        max_iterations=10,
    )

    return SupportTriageAgent(config)
