package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixURL(t *testing.T) {
	t.Parallel()

	t.Run("Remove Quotes and Spaces", func(t *testing.T) {
		url := "\"   http://example.com/path   \""
		expected := "http://example.com/path"
		assert.Equal(t, expected, FixURL(url))
	})

	t.Run("Remove Unbalanced Quotes", func(t *testing.T) {
		url := "http://example.com/path?q=\"value"
		expected := "http://example.com/path?q="
		assert.Equal(t, expected, FixURL(url))
	})

	t.Run("Remove Unbalanced Brackets", func(t *testing.T) {
		url := "http://example.com/path?q=[value"
		expected := "http://example.com/path?q="
		assert.Equal(t, expected, FixURL(url))
	})

	t.Run("Remove Trailing Semicolon", func(t *testing.T) {
		url := "http://example.com/path?q=value;"
		expected := "http://example.com/path?q=value"
		assert.Equal(t, expected, FixURL(url))
	})

	t.Run("Handle Escaped Characters", func(t *testing.T) {
		url := "http://example.com/path%20with%20spaces"
		expected := "http://example.com/path with spaces"
		assert.Equal(t, expected, FixURL(url))
	})
}

func TestFindUnbalancedQuote(t *testing.T) {
	t.Run("Balanced Quotes", func(t *testing.T) {
		s := "text with 'balanced' quotes"
		assert.Equal(t, -1, findUnbalancedQuote(s, '\''))
	})

	t.Run("Unbalanced Quotes", func(t *testing.T) {
		s := "text with 'unbalanced quotes"
		assert.Equal(t, 10, findUnbalancedQuote(s, '\''))
	})
}

func TestFindUnbalancedBracket(t *testing.T) {
	t.Parallel()

	t.Run("Balanced Brackets", func(t *testing.T) {
		s := "text with [balanced] brackets"
		assert.Equal(t, -1, findUnbalancedBracket(s, '[', ']'))
	})

	t.Run("Unbalanced Opening Bracket", func(t *testing.T) {
		s := "text with [unbalanced brackets"
		assert.Equal(t, 10, findUnbalancedBracket(s, '[', ']'))
	})

	t.Run("Unbalanced Closing Bracket", func(t *testing.T) {
		s := "text with unbalanced] brackets"
		assert.Equal(t, 20, findUnbalancedBracket(s, '[', ']'))
	})
}
