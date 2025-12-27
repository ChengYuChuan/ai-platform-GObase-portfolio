"""Text chunking utilities for document processing."""

import re
from dataclasses import dataclass

from langchain.text_splitter import RecursiveCharacterTextSplitter

from src.core import get_logger

logger = get_logger(__name__)


@dataclass
class ChunkingConfig:
    """Configuration for text chunking."""

    chunk_size: int = 1000
    chunk_overlap: int = 200
    length_function: str = "len"  # "len" or "tokens"
    separators: list[str] | None = None


class TextChunker:
    """Split text into overlapping chunks for embedding."""

    def __init__(
        self,
        chunk_size: int = 1000,
        chunk_overlap: int = 200,
        separators: list[str] | None = None,
    ) -> None:
        """Initialize the text chunker.

        Args:
            chunk_size: Maximum size of each chunk.
            chunk_overlap: Number of characters to overlap between chunks.
            separators: List of separators to use for splitting.
        """
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap

        # Default separators prioritize semantic boundaries
        self.separators = separators or [
            "\n\n\n",  # Multiple newlines (major section breaks)
            "\n\n",  # Paragraph breaks
            "\n",  # Line breaks
            ". ",  # Sentence endings
            "! ",
            "? ",
            "; ",  # Clause endings
            ", ",  # Clause breaks
            " ",  # Word breaks
            "",  # Character level (last resort)
        ]

        self._splitter = RecursiveCharacterTextSplitter(
            chunk_size=chunk_size,
            chunk_overlap=chunk_overlap,
            length_function=len,
            separators=self.separators,
            keep_separator=True,
        )

    def split_text(self, text: str) -> list[str]:
        """Split text into chunks.

        Args:
            text: Input text to split.

        Returns:
            List of text chunks.
        """
        if not text or not text.strip():
            return []

        # Clean the text
        cleaned_text = self._clean_text(text)

        # Use LangChain's splitter
        chunks = self._splitter.split_text(cleaned_text)

        # Post-process chunks
        processed_chunks = [self._post_process_chunk(chunk) for chunk in chunks]

        # Filter out empty chunks
        filtered_chunks = [c for c in processed_chunks if c.strip()]

        logger.debug(
            "Text chunked",
            original_length=len(text),
            num_chunks=len(filtered_chunks),
        )

        return filtered_chunks

    def _clean_text(self, text: str) -> str:
        """Clean and normalize text before chunking."""
        # Replace multiple spaces with single space
        text = re.sub(r" +", " ", text)

        # Replace multiple newlines with double newline
        text = re.sub(r"\n{3,}", "\n\n", text)

        # Remove leading/trailing whitespace from lines
        lines = [line.strip() for line in text.split("\n")]
        text = "\n".join(lines)

        # Remove null bytes and other control characters
        text = re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", "", text)

        return text.strip()

    def _post_process_chunk(self, chunk: str) -> str:
        """Post-process a chunk to ensure quality."""
        # Strip whitespace
        chunk = chunk.strip()

        # Ensure chunk doesn't start with orphaned punctuation
        while chunk and chunk[0] in ".,;:!?)]}":
            chunk = chunk[1:].strip()

        return chunk


class SemanticChunker:
    """Advanced chunker that considers semantic boundaries."""

    def __init__(
        self,
        chunk_size: int = 1000,
        chunk_overlap: int = 200,
    ) -> None:
        """Initialize the semantic chunker."""
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap
        self.base_chunker = TextChunker(chunk_size, chunk_overlap)

    def split_text(self, text: str, preserve_headers: bool = True) -> list[str]:
        """Split text while preserving semantic structure.

        Args:
            text: Input text to split.
            preserve_headers: Whether to keep headers with their content.

        Returns:
            List of semantically meaningful chunks.
        """
        if not preserve_headers:
            return self.base_chunker.split_text(text)

        # Detect markdown headers
        header_pattern = re.compile(r"^(#{1,6})\s+(.+)$", re.MULTILINE)

        # Find all headers and their positions
        headers = list(header_pattern.finditer(text))

        if not headers:
            return self.base_chunker.split_text(text)

        chunks = []
        prev_end = 0

        for i, header_match in enumerate(headers):
            # Content before this header
            if header_match.start() > prev_end:
                pre_content = text[prev_end : header_match.start()].strip()
                if pre_content:
                    chunks.extend(self.base_chunker.split_text(pre_content))

            # Get the section content (from this header to next header or end)
            next_start = headers[i + 1].start() if i + 1 < len(headers) else len(text)
            section = text[header_match.start() : next_start].strip()

            # If section is small enough, keep it as one chunk
            if len(section) <= self.chunk_size:
                chunks.append(section)
            else:
                # Split section but prepend header to each chunk
                header_text = header_match.group(0)
                section_content = text[header_match.end() : next_start].strip()

                sub_chunks = self.base_chunker.split_text(section_content)
                for j, sub_chunk in enumerate(sub_chunks):
                    if j == 0:
                        chunks.append(f"{header_text}\n\n{sub_chunk}")
                    else:
                        # Add context header for subsequent chunks
                        chunks.append(f"[Continued: {header_match.group(2)}]\n\n{sub_chunk}")

            prev_end = next_start

        return chunks


class CodeChunker:
    """Specialized chunker for code files."""

    def __init__(
        self,
        chunk_size: int = 1500,
        chunk_overlap: int = 100,
    ) -> None:
        """Initialize the code chunker."""
        self.chunk_size = chunk_size
        self.chunk_overlap = chunk_overlap

    def split_code(self, code: str, language: str = "python") -> list[str]:
        """Split code while preserving function/class boundaries.

        Args:
            code: Source code to split.
            language: Programming language.

        Returns:
            List of code chunks.
        """
        if language == "python":
            return self._split_python(code)
        else:
            # Fallback to line-based splitting
            return self._split_by_lines(code)

    def _split_python(self, code: str) -> list[str]:
        """Split Python code by function/class definitions."""
        # Pattern to match function and class definitions
        definition_pattern = re.compile(
            r"^((?:async\s+)?def\s+\w+|class\s+\w+)", re.MULTILINE
        )

        matches = list(definition_pattern.finditer(code))

        if not matches:
            return self._split_by_lines(code)

        chunks = []
        prev_end = 0

        for i, match in enumerate(matches):
            # Content before this definition (module-level code)
            if match.start() > prev_end and i == 0:
                pre_content = code[prev_end : match.start()].strip()
                if pre_content:
                    chunks.append(pre_content)

            # Get the definition content
            next_start = matches[i + 1].start() if i + 1 < len(matches) else len(code)
            definition = code[match.start() : next_start].strip()

            if len(definition) <= self.chunk_size:
                chunks.append(definition)
            else:
                # Split large definitions
                sub_chunks = self._split_by_lines(definition)
                chunks.extend(sub_chunks)

            prev_end = next_start

        return chunks

    def _split_by_lines(self, code: str) -> list[str]:
        """Split code by lines while respecting chunk size."""
        lines = code.split("\n")
        chunks = []
        current_chunk: list[str] = []
        current_size = 0

        for line in lines:
            line_size = len(line) + 1  # +1 for newline

            if current_size + line_size > self.chunk_size and current_chunk:
                chunks.append("\n".join(current_chunk))
                # Keep overlap
                overlap_lines = current_chunk[-(self.chunk_overlap // 50) :] if self.chunk_overlap else []
                current_chunk = overlap_lines
                current_size = sum(len(l) + 1 for l in current_chunk)

            current_chunk.append(line)
            current_size += line_size

        if current_chunk:
            chunks.append("\n".join(current_chunk))

        return chunks
