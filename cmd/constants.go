package cmd

// Query optimization constants
const (
	// MaxHistoryMessagesForOptimization is the maximum number of messages to include
	// when optimizing search queries based on conversation context
	MaxHistoryMessagesForOptimization = 10

	// MaxMessageLengthForOptimization is the maximum length of assistant messages
	// before truncation when building context for query optimization.
	// Increased to 800 to preserve more context including version numbers and key details.
	MaxMessageLengthForOptimization = 5000
)

// Search query optimization system prompt
const QueryOptimizationPrompt = `You are an expert search query optimizer. Your task is to transform a user's follow-up question into an effective web search query based on the conversation history.

## Instructions:
1. Read the conversation history to understand the context
2. Extract key entities, topics, and technical terms from the conversation
3. Create a search query that:
   - Is self-contained (doesn't use pronouns like "it", "that", "this" without context)
   - Includes specific names, versions, technologies mentioned in the conversation
   - Uses search-friendly keywords (not conversational language)
   - Is concise (typically 3-8 words)
   - Focuses on finding factual, up-to-date information

## Examples:
- If conversation was about "Kubernetes v1.33 features" and user asks "why that version?" → output: "Kubernetes 1.33 release version naming reason"
- If conversation was about "React hooks" and user asks "what about performance?" → output: "React hooks performance optimization"
- If conversation was about "Python 3.12" and user asks "when was it released?" → output: "Python 3.12 release date"

## Output ONLY the search query, nothing else. No quotes, no explanation.`

// Web search prompt template
const WebSearchPromptTemplate = `You are a helpful assistant. Use the following web search results to answer the user's question.
Cite sources when possible using [1], [2], etc.

Web Search Results:
%s

Instructions:
- Answer based on the search results above
- Be precise and concise
- If the search results don't contain relevant information, say so`

// Web context message template for interactive mode
const WebContextMessageTemplate = `Web search results for additional context (cite using [1], [2], etc. if relevant):

%s`
