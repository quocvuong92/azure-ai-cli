package executor

import (
	"sync"
)

// PermissionManager handles command execution permissions
type PermissionManager struct {
	mu               sync.RWMutex
	alwaysAllow      map[string]bool
	dangerousEnabled bool
	autoAllowReads   bool
}

// NewPermissionManager creates a new permission manager with safe defaults
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{
		alwaysAllow:    make(map[string]bool),
		autoAllowReads: true, // Default: auto-allow safe read-only commands
	}
}

// CheckPermission checks if a command is allowed to execute
// Returns: (allowed, needsConfirm, reason)
func (pm *PermissionManager) CheckPermission(cmd string) (allowed bool, needsConfirm bool, reason string) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Check if user previously said "always allow" for this specific command
	if pm.alwaysAllow[cmd] {
		return true, false, "Previously approved by user"
	}

	risk := ClassifyCommand(cmd)

	switch risk {
	case Safe:
		if pm.autoAllowReads {
			return true, false, "Safe read-only command"
		}
		return false, true, "Needs confirmation"

	case NeedsConfirm:
		return false, true, "Command may modify system state"

	case Dangerous:
		if pm.dangerousEnabled {
			return false, true, "Dangerous command (requires explicit confirmation)"
		}
		return false, false, "Dangerous command blocked (use /allow-dangerous to enable)"
	}

	return false, true, "Unknown command type"
}

// AddToAllowlist adds a command to the always-allow list
func (pm *PermissionManager) AddToAllowlist(cmd string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alwaysAllow[cmd] = true
}

// EnableDangerous enables execution of dangerous commands (with confirmation)
func (pm *PermissionManager) EnableDangerous() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.dangerousEnabled = true
}

// DisableDangerous disables execution of dangerous commands
func (pm *PermissionManager) DisableDangerous() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.dangerousEnabled = false
}

// SetAutoAllowReads sets whether to auto-allow safe read-only commands
func (pm *PermissionManager) SetAutoAllowReads(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.autoAllowReads = enabled
}

// GetSettings returns current permission settings
func (pm *PermissionManager) GetSettings() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return map[string]interface{}{
		"auto_allow_reads":  pm.autoAllowReads,
		"dangerous_enabled": pm.dangerousEnabled,
		"allowlist_count":   len(pm.alwaysAllow),
	}
}

// ClearAllowlist clears all previously approved commands
func (pm *PermissionManager) ClearAllowlist() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alwaysAllow = make(map[string]bool)
}
