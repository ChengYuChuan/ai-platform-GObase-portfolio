"""Tests for text chunking module."""

import pytest

from src.rag.ingestion.chunker import (
    TextChunker,
    SemanticChunker,
    CodeChunker,
)


class TestTextChunker:
    """Tests for TextChunker class."""

    def test_init_default_params(self):
        """Test initialization with default parameters."""
        chunker = TextChunker()
        assert chunker.chunk_size == 1000
        assert chunker.chunk_overlap == 200

    def test_init_custom_params(self):
        """Test initialization with custom parameters."""
        chunker = TextChunker(chunk_size=500, chunk_overlap=50)
        assert chunker.chunk_size == 500
        assert chunker.chunk_overlap == 50

    def test_split_empty_text(self):
        """Test splitting empty text returns empty list."""
        chunker = TextChunker()
        result = chunker.split_text("")
        assert result == []

    def test_split_whitespace_only(self):
        """Test splitting whitespace-only text returns empty list."""
        chunker = TextChunker()
        result = chunker.split_text("   \n\n   \t\t   ")
        assert result == []

    def test_split_short_text(self):
        """Test splitting text shorter than chunk size."""
        chunker = TextChunker(chunk_size=1000)
        text = "This is a short text."
        result = chunker.split_text(text)
        assert len(result) == 1
        assert result[0] == text

    def test_split_long_text(self):
        """Test splitting text longer than chunk size."""
        chunker = TextChunker(chunk_size=100, chunk_overlap=20)
        text = "This is a sentence. " * 20  # About 400 chars
        result = chunker.split_text(text)
        assert len(result) > 1

    def test_split_preserves_content(self):
        """Test that splitting preserves all content."""
        chunker = TextChunker(chunk_size=50, chunk_overlap=10)
        text = "The quick brown fox jumps over the lazy dog. " * 5
        result = chunker.split_text(text)

        # Combine chunks and verify key content is present
        combined = " ".join(result)
        assert "quick brown fox" in combined
        assert "lazy dog" in combined

    def test_clean_text_removes_extra_spaces(self):
        """Test that extra spaces are normalized."""
        chunker = TextChunker()
        text = "Hello    world.     This   is   a   test."
        cleaned = chunker._clean_text(text)
        assert "    " not in cleaned

    def test_clean_text_normalizes_newlines(self):
        """Test that multiple newlines are normalized."""
        chunker = TextChunker()
        text = "Paragraph 1.\n\n\n\n\nParagraph 2."
        cleaned = chunker._clean_text(text)
        assert "\n\n\n" not in cleaned


class TestSemanticChunker:
    """Tests for SemanticChunker class."""

    def test_init(self):
        """Test initialization."""
        chunker = SemanticChunker(chunk_size=500, chunk_overlap=50)
        assert chunker.chunk_size == 500
        assert chunker.chunk_overlap == 50

    def test_split_without_headers(self):
        """Test splitting text without headers."""
        # Need chunk_overlap < chunk_size
        chunker = SemanticChunker(chunk_size=300, chunk_overlap=50)
        text = "This is regular text without any markdown headers. " * 10
        # When preserve_headers=True but there are no headers, falls back to base chunker
        result = chunker.split_text(text, preserve_headers=True)
        assert len(result) >= 1

    def test_split_with_headers(self):
        """Test splitting text with markdown headers."""
        chunker = SemanticChunker(chunk_size=200)
        text = """# Introduction

This is the introduction section.

## Section 1

This is section 1 content.

## Section 2

This is section 2 content.
"""
        result = chunker.split_text(text, preserve_headers=True)
        assert len(result) >= 1
        # Headers should be preserved in chunks
        assert any("Introduction" in chunk for chunk in result)


class TestCodeChunker:
    """Tests for CodeChunker class."""

    def test_init(self):
        """Test initialization."""
        chunker = CodeChunker(chunk_size=1000)
        assert chunker.chunk_size == 1000

    def test_split_python_code(self):
        """Test splitting Python code."""
        chunker = CodeChunker(chunk_size=500)
        code = '''
def hello():
    """Say hello."""
    print("Hello, world!")

def goodbye():
    """Say goodbye."""
    print("Goodbye, world!")

class Greeter:
    """A greeter class."""

    def greet(self, name):
        print(f"Hello, {name}!")
'''
        result = chunker.split_code(code, language="python")
        assert len(result) >= 1

    def test_split_preserves_functions(self):
        """Test that function definitions are preserved."""
        chunker = CodeChunker(chunk_size=200)
        code = '''
def short_function():
    return 42

def another_function():
    return "hello"
'''
        result = chunker.split_code(code, language="python")

        # Check that function definitions are in chunks
        combined = "\n".join(result)
        assert "def short_function" in combined
        assert "def another_function" in combined

    def test_split_unknown_language(self):
        """Test splitting unknown language falls back to line-based."""
        chunker = CodeChunker(chunk_size=100)
        code = "some random code\nmore code\neven more"
        result = chunker.split_code(code, language="unknown")
        assert len(result) >= 1
