std = "lua51"
max_line_length = false
globals = { "WoWClaudeDB", "WoWClaudeInbox", "SLASH_WOWCLAUDE1", "SLASH_WOWCLAUDE2", "SlashCmdList" }
read_globals = { "CreateFrame", "UIParent", "UISpecialFrames", "ReloadUI", "ChatFontNormal", "tinsert", "strtrim", "print" }
files["tests"] = { globals = { "arg" } }
