package sources

import (
	"net/url"
	"strings"
)

func FixURL(URL string) (fixedURL string) {
	fixedURL = URL

	// remove beginning and ending quotes
	fixedURL = strings.Trim(fixedURL, "\"")
	fixedURL = strings.Trim(fixedURL, "'")

	fixedURL, _ = url.QueryUnescape(fixedURL)

	// remove beginning and ending spaces
	fixedURL = strings.Trim(fixedURL, " ")

	// ',",`,
	quotes := []rune{'\'', '"', '`'}

	for i := range quotes {
		quote := quotes[i]

		indexOfQuote := findUnbalancedQuote(URL, quote)
		if indexOfQuote <= len(fixedURL) && indexOfQuote >= 0 {
			fixedURL = fixedURL[:indexOfQuote]
		}
	}

	// (),[],{}
	parentheses := []struct {
		Opening, Closing rune
	}{
		{'[', ']'},
		{'(', ')'},
		{'{', '}'},
	}

	for i := range parentheses {
		parenthesis := parentheses[i]

		indexOfParenthesis := findUnbalancedBracket(URL, parenthesis.Opening, parenthesis.Closing)
		if indexOfParenthesis <= len(fixedURL) && indexOfParenthesis >= 0 {
			fixedURL = fixedURL[:indexOfParenthesis]
		}
	}

	// ;
	indexOfComma := strings.Index(fixedURL, ";")
	if indexOfComma <= len(fixedURL) && indexOfComma >= 0 {
		fixedURL = fixedURL[:indexOfComma]
	}

	return
}

func findUnbalancedQuote(s string, quoteChar rune) int {
	insideQuotes := false

	for _, ch := range s {
		if ch == quoteChar {
			if insideQuotes {
				insideQuotes = false
			} else {
				insideQuotes = true
			}
		}
	}

	// If still inside quotes at the end of the string,
	// find the index of the opening quote
	if insideQuotes {
		for i, ch := range s {
			if ch == quoteChar {
				return i
			}
		}
	}

	return -1 // return -1 if all quotes are balanced
}

func findUnbalancedBracket(s string, openChar, closeChar rune) int {
	openCount := 0

	var firstOpenIndex int

	for i, ch := range s {
		if ch == openChar {
			if openCount == 0 {
				firstOpenIndex = i
			}

			openCount++
		} else if ch == closeChar {
			openCount--

			if openCount < 0 {
				return i // Found an unbalanced closing bracket
			}
		}
	}

	// If there are unmatched opening brackets
	if openCount > 0 {
		return firstOpenIndex
	}

	return -1 // All brackets are balanced
}
