"""Data processing tools for agents."""

import json
import re
from typing import Any

from src.agents.tools.base import BaseTool, ToolResult
from src.core import get_logger
from src.core.config import get_settings

logger = get_logger(__name__)


class ExtractDataTool(BaseTool):
    """Extract structured data from unstructured text."""

    name = "extract_data"
    description = """Extract structured data (like names, dates, amounts, etc.)
    from unstructured text content. Returns data in a structured format."""

    async def execute(
        self,
        text: str,
        extraction_schema: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> ToolResult:
        """Extract data from text.

        Args:
            text: Text to extract data from.
            extraction_schema: Optional schema defining what to extract.

        Returns:
            ToolResult with extracted data.
        """
        try:
            from langchain_openai import ChatOpenAI
            from langchain_core.prompts import ChatPromptTemplate

            settings = get_settings()

            llm = ChatOpenAI(
                model="gpt-4o-mini",
                temperature=0,
                openai_api_key=settings.openai_api_key,
            )

            if extraction_schema:
                schema_str = json.dumps(extraction_schema, indent=2)
                prompt = ChatPromptTemplate.from_template(
                    """Extract data from the following text according to this schema:
{schema}

Text:
{text}

Return the extracted data as valid JSON matching the schema.
If a field cannot be found, use null.

Extracted JSON:"""
                )
                result = await (prompt | llm).ainvoke({
                    "schema": schema_str,
                    "text": text,
                })
            else:
                # Auto-detect fields to extract
                prompt = ChatPromptTemplate.from_template(
                    """Extract all structured data from the following text.
Look for: names, dates, amounts, addresses, phone numbers, emails,
and any other identifiable data points.

Text:
{text}

Return the extracted data as valid JSON.

Extracted JSON:"""
                )
                result = await (prompt | llm).ainvoke({"text": text})

            # Parse the JSON response
            try:
                # Clean the response
                json_str = result.content.strip()
                if json_str.startswith("```"):
                    json_str = re.sub(r"^```(?:json)?\n?", "", json_str)
                    json_str = re.sub(r"\n?```$", "", json_str)

                extracted_data = json.loads(json_str)
            except json.JSONDecodeError:
                # If JSON parsing fails, return raw text
                extracted_data = {"raw_extraction": result.content}

            return ToolResult.success(
                extracted_data,
                text_length=len(text),
            )

        except Exception as e:
            logger.error("Data extraction failed", error=str(e))
            return ToolResult.error(f"Extraction failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Text to extract data from",
                    },
                    "extraction_schema": {
                        "type": "object",
                        "description": "Optional schema defining fields to extract",
                    },
                },
                "required": ["text"],
            },
        }


class ValidateDataTool(BaseTool):
    """Validate data against rules or schema."""

    name = "validate_data"
    description = """Validate data against specified rules or schema.
    Returns validation results with any errors found."""

    # Common validation patterns
    PATTERNS = {
        "email": r"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$",
        "phone": r"^\+?1?\d{9,15}$",
        "date_iso": r"^\d{4}-\d{2}-\d{2}$",
        "url": r"^https?://[^\s]+$",
        "number": r"^-?\d+\.?\d*$",
    }

    async def execute(
        self,
        data: dict[str, Any],
        rules: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> ToolResult:
        """Validate data.

        Args:
            data: Data to validate.
            rules: Validation rules.

        Returns:
            ToolResult with validation results.
        """
        try:
            errors: list[dict[str, str]] = []
            warnings: list[dict[str, str]] = []

            if rules:
                for field, rule in rules.items():
                    value = data.get(field)

                    # Required check
                    if rule.get("required", False) and value is None:
                        errors.append({
                            "field": field,
                            "error": "Required field is missing",
                        })
                        continue

                    if value is None:
                        continue

                    # Type check
                    expected_type = rule.get("type")
                    if expected_type:
                        type_map = {
                            "string": str,
                            "number": (int, float),
                            "integer": int,
                            "boolean": bool,
                            "array": list,
                            "object": dict,
                        }
                        if expected_type in type_map:
                            if not isinstance(value, type_map[expected_type]):
                                errors.append({
                                    "field": field,
                                    "error": f"Expected {expected_type}, got {type(value).__name__}",
                                })

                    # Pattern check
                    pattern = rule.get("pattern")
                    if pattern and isinstance(value, str):
                        if pattern in self.PATTERNS:
                            pattern = self.PATTERNS[pattern]
                        if not re.match(pattern, value):
                            errors.append({
                                "field": field,
                                "error": f"Value does not match pattern: {pattern}",
                            })

                    # Min/max checks
                    if "min" in rule:
                        if isinstance(value, (int, float)) and value < rule["min"]:
                            errors.append({
                                "field": field,
                                "error": f"Value {value} is less than minimum {rule['min']}",
                            })
                        elif isinstance(value, str) and len(value) < rule["min"]:
                            errors.append({
                                "field": field,
                                "error": f"Length {len(value)} is less than minimum {rule['min']}",
                            })

                    if "max" in rule:
                        if isinstance(value, (int, float)) and value > rule["max"]:
                            errors.append({
                                "field": field,
                                "error": f"Value {value} is greater than maximum {rule['max']}",
                            })
                        elif isinstance(value, str) and len(value) > rule["max"]:
                            errors.append({
                                "field": field,
                                "error": f"Length {len(value)} is greater than maximum {rule['max']}",
                            })

                    # Enum check
                    if "enum" in rule and value not in rule["enum"]:
                        errors.append({
                            "field": field,
                            "error": f"Value must be one of: {rule['enum']}",
                        })

            is_valid = len(errors) == 0

            return ToolResult.success(
                {
                    "valid": is_valid,
                    "errors": errors,
                    "warnings": warnings,
                    "fields_checked": len(rules) if rules else 0,
                },
            )

        except Exception as e:
            logger.error("Data validation failed", error=str(e))
            return ToolResult.error(f"Validation failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "data": {
                        "type": "object",
                        "description": "Data to validate",
                    },
                    "rules": {
                        "type": "object",
                        "description": "Validation rules for each field",
                    },
                },
                "required": ["data"],
            },
        }


class TransformDataTool(BaseTool):
    """Transform data from one format to another."""

    name = "transform_data"
    description = """Transform data from one format or structure to another.
    Supports field mapping, type conversion, and format changes."""

    async def execute(
        self,
        data: dict[str, Any],
        transformations: list[dict[str, Any]],
        **kwargs: Any,
    ) -> ToolResult:
        """Transform data.

        Args:
            data: Input data to transform.
            transformations: List of transformations to apply.

        Returns:
            ToolResult with transformed data.
        """
        try:
            result = dict(data)

            for transform in transformations:
                transform_type = transform.get("type")

                if transform_type == "rename":
                    # Rename a field
                    old_name = transform.get("from")
                    new_name = transform.get("to")
                    if old_name in result:
                        result[new_name] = result.pop(old_name)

                elif transform_type == "convert":
                    # Convert field type
                    field = transform.get("field")
                    to_type = transform.get("to_type")
                    if field in result:
                        value = result[field]
                        if to_type == "string":
                            result[field] = str(value)
                        elif to_type == "integer":
                            result[field] = int(value)
                        elif to_type == "float":
                            result[field] = float(value)
                        elif to_type == "boolean":
                            result[field] = bool(value)

                elif transform_type == "format":
                    # Format a field
                    field = transform.get("field")
                    format_spec = transform.get("format")
                    if field in result:
                        value = result[field]
                        if format_spec == "uppercase":
                            result[field] = str(value).upper()
                        elif format_spec == "lowercase":
                            result[field] = str(value).lower()
                        elif format_spec == "title":
                            result[field] = str(value).title()
                        elif format_spec == "trim":
                            result[field] = str(value).strip()

                elif transform_type == "remove":
                    # Remove a field
                    field = transform.get("field")
                    result.pop(field, None)

                elif transform_type == "add":
                    # Add a new field
                    field = transform.get("field")
                    value = transform.get("value")
                    result[field] = value

                elif transform_type == "merge":
                    # Merge multiple fields
                    fields = transform.get("fields", [])
                    target = transform.get("target")
                    separator = transform.get("separator", " ")
                    values = [str(result.get(f, "")) for f in fields]
                    result[target] = separator.join(v for v in values if v)

            return ToolResult.success(
                result,
                transformations_applied=len(transformations),
            )

        except Exception as e:
            logger.error("Data transformation failed", error=str(e))
            return ToolResult.error(f"Transformation failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "data": {
                        "type": "object",
                        "description": "Input data to transform",
                    },
                    "transformations": {
                        "type": "array",
                        "description": "List of transformations to apply",
                        "items": {
                            "type": "object",
                            "properties": {
                                "type": {
                                    "type": "string",
                                    "enum": ["rename", "convert", "format", "remove", "add", "merge"],
                                },
                            },
                        },
                    },
                },
                "required": ["data", "transformations"],
            },
        }


class FormatOutputTool(BaseTool):
    """Format data for output in various formats."""

    name = "format_output"
    description = """Format data for output in various formats like JSON,
    CSV, markdown table, or plain text."""

    async def execute(
        self,
        data: Any,
        format_type: str = "json",
        **kwargs: Any,
    ) -> ToolResult:
        """Format data for output.

        Args:
            data: Data to format.
            format_type: Output format (json, csv, markdown, text).

        Returns:
            ToolResult with formatted output.
        """
        try:
            if format_type == "json":
                output = json.dumps(data, indent=2, ensure_ascii=False)

            elif format_type == "csv":
                if isinstance(data, list) and data:
                    # Assume list of dicts
                    if isinstance(data[0], dict):
                        headers = list(data[0].keys())
                        lines = [",".join(headers)]
                        for row in data:
                            values = [str(row.get(h, "")).replace(",", ";") for h in headers]
                            lines.append(",".join(values))
                        output = "\n".join(lines)
                    else:
                        output = "\n".join(str(item) for item in data)
                else:
                    output = str(data)

            elif format_type == "markdown":
                if isinstance(data, list) and data and isinstance(data[0], dict):
                    # Create markdown table
                    headers = list(data[0].keys())
                    lines = [
                        "| " + " | ".join(headers) + " |",
                        "| " + " | ".join(["---"] * len(headers)) + " |",
                    ]
                    for row in data:
                        values = [str(row.get(h, "")) for h in headers]
                        lines.append("| " + " | ".join(values) + " |")
                    output = "\n".join(lines)
                elif isinstance(data, dict):
                    lines = []
                    for key, value in data.items():
                        lines.append(f"**{key}**: {value}")
                    output = "\n".join(lines)
                else:
                    output = str(data)

            elif format_type == "text":
                if isinstance(data, dict):
                    lines = []
                    for key, value in data.items():
                        lines.append(f"{key}: {value}")
                    output = "\n".join(lines)
                elif isinstance(data, list):
                    output = "\n".join(str(item) for item in data)
                else:
                    output = str(data)

            else:
                output = str(data)

            return ToolResult.success(
                {"formatted_output": output, "format": format_type},
            )

        except Exception as e:
            logger.error("Output formatting failed", error=str(e))
            return ToolResult.error(f"Formatting failed: {str(e)}")

    def get_schema(self) -> dict[str, Any]:
        """Get the tool schema."""
        return {
            "name": self.name,
            "description": self.description,
            "parameters": {
                "type": "object",
                "properties": {
                    "data": {
                        "description": "Data to format",
                    },
                    "format_type": {
                        "type": "string",
                        "enum": ["json", "csv", "markdown", "text"],
                        "default": "json",
                    },
                },
                "required": ["data"],
            },
        }
