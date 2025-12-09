package api

// ExecuteCommandTool is the tool definition for command execution
var ExecuteCommandTool = Tool{
	Type: "function",
	Function: Function{
		Name:        "execute_command",
		Description: "Execute a shell command in the user's terminal and return the output. Use this to help users with system tasks, file operations, git commands, package management, and other terminal operations. The command will run in the user's current working directory.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute (e.g., 'ls -la', 'git status', 'npm install')",
				},
				"reasoning": map[string]interface{}{
					"type":        "string",
					"description": "Brief explanation of why this command is needed to accomplish the user's request",
				},
			},
			"required": []string{"command", "reasoning"},
		},
	},
}

// GetDefaultTools returns the default set of tools available to the AI
func GetDefaultTools() []Tool {
	return []Tool{
		ExecuteCommandTool,
	}
}
