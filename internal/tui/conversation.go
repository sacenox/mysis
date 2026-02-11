package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/styles"
)

// Conversation manages the conversation log viewport.
type Conversation struct {
	viewport viewport.Model
	messages []provider.Message
	width    int
	height   int
}

// NewConversation creates a new conversation viewport.
func NewConversation(width, height int) Conversation {
	vp := viewport.New(width, height)
	vp.Style = LogStyle

	// Ensure viewport fills with background color
	vp.SetContent("")

	return Conversation{
		viewport: vp,
		messages: []provider.Message{},
		width:    width,
		height:   height,
	}
}

// SetSize updates the viewport size.
func (c *Conversation) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
	// Re-render content with new width
	c.updateContent()
}

// SetMessages updates the conversation messages and re-renders.
func (c *Conversation) SetMessages(messages []provider.Message) {
	c.messages = messages
	c.updateContent()
}

// AddMessage appends a message and re-renders.
func (c *Conversation) AddMessage(msg provider.Message) {
	c.messages = append(c.messages, msg)
	c.updateContent()
}

// updateContent renders all messages and sets viewport content.
func (c *Conversation) updateContent() {
	if len(c.messages) == 0 {
		c.viewport.SetContent(DimmedStyle.Render("No conversation history."))
		return
	}

	// Remember if user was at bottom before updating
	wasAtBottom := c.viewport.AtBottom()

	var lines []string
	for _, msg := range c.messages {
		lines = append(lines, c.renderMessage(msg)...)
		// Blank line with background - must fill width
		blankStyle := lipgloss.NewStyle().
			Background(styles.ColorBg).
			Width(c.width)
		lines = append(lines, blankStyle.Render(""))
	}

	content := strings.Join(lines, "\n")
	c.viewport.SetContent(content)

	// Auto-scroll to bottom if user was already there
	if wasAtBottom {
		c.viewport.GotoBottom()
	}
}

// renderMessage renders a single message with role, content, and tool calls.
func (c Conversation) renderMessage(msg provider.Message) []string {
	var lines []string

	// Timestamp first, then role label
	var roleLabelText string

	// Add timestamp if present
	if !msg.CreatedAt.IsZero() {
		timestamp := msg.CreatedAt.Format("15:04:05")
		timestampStyled := DimmedStyle.Render("[" + timestamp + "] ")
		roleLabelText = timestampStyled
	}

	// Add role label
	roleLabelText += RoleLabel(msg.Role)

	roleLineStyle := lipgloss.NewStyle().
		Background(styles.ColorBg).
		Width(c.width)
	lines = append(lines, roleLineStyle.Render(roleLabelText))

	// Reasoning (if present, for assistant messages)
	if msg.Reasoning != "" {
		reasoningLines := c.renderReasoning(msg.Reasoning)
		lines = append(lines, reasoningLines...)
	}

	// Content (if present)
	if msg.Content != "" {
		contentLines := c.renderContent(msg.Content, msg.Role)
		lines = append(lines, contentLines...)
	}

	// Tool calls (if present)
	if len(msg.ToolCalls) > 0 {
		toolLines := c.renderToolCalls(msg.ToolCalls)
		lines = append(lines, toolLines...)
	}

	return lines
}

// truncateContent truncates content similar to CLI behavior.
// - Tool results: truncated to 100 chars
// - Reasoning: truncated to 200 chars
// - Regular content: no truncation (but word-wrapped)
func (c Conversation) truncateContent(content string, role string) string {
	// Tool results get truncated aggressively (like CLI)
	if role == "tool" {
		if len(content) > 100 {
			return content[:97] + "..."
		}
		return content
	}

	// No truncation for other roles, just return as-is
	// (word wrapping handles long content)
	return content
}

// renderReasoning renders reasoning/thinking content with truncation.
// Per design spec: truncate from end (show last 200 chars), no word wrap.
func (c Conversation) renderReasoning(reasoning string) []string {
	// Trim and collapse whitespace
	reasoning = strings.TrimSpace(reasoning)
	reasoning = strings.Join(strings.Fields(reasoning), " ")

	// Truncate from end if too long (keep last 200 chars)
	if len(reasoning) > 200 {
		reasoning = "..." + reasoning[len(reasoning)-197:]
	}

	// Apply dimmed style with symbol prefix
	style := DimmedStyle.Width(c.width)
	return []string{style.Render("  ∴ " + reasoning)}
}

// renderContent renders message content with truncation but no word wrap per design spec.
func (c Conversation) renderContent(content string, role string) []string {
	// Truncate content based on role (like CLI does)
	truncated := c.truncateContent(content, role)

	// Apply role style with background to entire line including padding
	// Set width to fill the viewport so background extends to edge
	// No word wrap per design spec: "No word wrap for reasoning, user messages, or agent replies"
	style := RoleStyle(role).Width(c.width)
	var lines []string

	// Split by newlines only (no word wrapping)
	for _, line := range strings.Split(truncated, "\n") {
		// Render padding + content with full width so background fills line
		lines = append(lines, style.Render("  "+line))
	}

	return lines
}

// renderToolCalls renders tool calls in a compact format.
func (c Conversation) renderToolCalls(toolCalls []provider.ToolCall) []string {
	var lines []string

	for _, tc := range toolCalls {
		// Build tool line content
		content := fmt.Sprintf("  ⚙ %s", tc.Name)

		// Arguments (truncated)
		var args map[string]interface{}
		if err := json.Unmarshal(tc.Arguments, &args); err == nil {
			argsJSON, _ := json.Marshal(args)
			argsStr := string(argsJSON)
			if len(argsStr) > 60 {
				argsStr = argsStr[:57] + "..."
			}
			content += " " + argsStr
		}

		// Render with width so background fills line
		toolStyle := ToolStyle.Width(c.width)
		lines = append(lines, toolStyle.Render(content))
	}

	return lines
}

// Update handles viewport updates (scrolling, etc).
func (c Conversation) Update(msg interface{}) (Conversation, bool) {
	var cmd interface{}
	c.viewport, cmd = c.viewport.Update(msg)
	return c, cmd != nil
}

// View renders the conversation viewport.
func (c Conversation) View() string {
	// The viewport already has LogStyle which includes background
	// Just return it directly - the parent will handle sizing
	return c.viewport.View()
}

// GotoBottom scrolls to the bottom.
func (c *Conversation) GotoBottom() {
	c.viewport.GotoBottom()
}

// ScrollPercent returns the current scroll percentage.
func (c Conversation) ScrollPercent() float64 {
	return c.viewport.ScrollPercent()
}
