local addonName, ns = ...
local Core = ns.Core

local ORANGE = "|cffd97757"
local CREAM = "|cffe8e3d8"
local DIM = "|cff8a857c"
local RESET = "|r"

local ROW_HEIGHT = 24
local SIDEBAR = 220
local MAX_ROWS = 15

local UI = {}
ns.UI = UI

local function backdrop(frame, r, g, b, a)
	frame:SetBackdrop({
		bgFile = "Interface\\Buttons\\WHITE8x8",
		edgeFile = "Interface\\Buttons\\WHITE8x8",
		edgeSize = 1,
	})
	frame:SetBackdropColor(r, g, b, a or 1)
	frame:SetBackdropBorderColor(0.28, 0.27, 0.25, 1)
end

local function button(parent, text, width, height)
	local b = CreateFrame("Button", nil, parent, "BackdropTemplate")
	b:SetSize(width, height)
	backdrop(b, 0.22, 0.21, 0.2)
	b.text = b:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
	b.text:SetPoint("CENTER")
	b.text:SetText(text)
	b:SetScript("OnEnter", function(self) self:SetBackdropColor(0.3, 0.28, 0.26) end)
	b:SetScript("OnLeave", function(self) self:SetBackdropColor(0.22, 0.21, 0.2) end)
	return b
end

local function shorten(s, n)
	s = (s or ""):gsub("\n", " ")
	if #s > n then
		return s:sub(1, n - 1) .. "…"
	end
	return s
end

function UI.Build()
	local f = CreateFrame("Frame", "WoWClaudeFrame", UIParent, "BackdropTemplate")
	UI.frame = f
	f:SetSize(760, 500)
	f:SetPoint("CENTER")
	f:SetFrameStrata("DIALOG")
	backdrop(f, 0.16, 0.15, 0.14, 0.97)
	f:SetMovable(true)
	f:EnableMouse(true)
	f:RegisterForDrag("LeftButton")
	f:SetScript("OnDragStart", f.StartMoving)
	f:SetScript("OnDragStop", f.StopMovingOrSizing)
	f:SetClampedToScreen(true)
	f:Hide()
	tinsert(UISpecialFrames, "WoWClaudeFrame")

	local title = f:CreateFontString(nil, "OVERLAY", "GameFontNormalLarge")
	title:SetPoint("TOPLEFT", 16, -12)
	title:SetText(ORANGE .. "✱ Claude" .. RESET)

	local close = button(f, "×", 24, 24)
	close:SetPoint("TOPRIGHT", -10, -10)
	close:SetScript("OnClick", function() f:Hide() end)

	local refresh = button(f, "Refresh", 70, 24)
	refresh:SetPoint("RIGHT", close, "LEFT", -6, 0)
	refresh:SetScript("OnClick", ReloadUI)

	f.status = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
	f.status:SetPoint("TOPLEFT", title, "BOTTOMLEFT", 0, -4)
	f.status:SetPoint("RIGHT", refresh, "LEFT", -10, 0)
	f.status:SetJustifyH("LEFT")

	local side = CreateFrame("Frame", nil, f, "BackdropTemplate")
	side:SetPoint("TOPLEFT", 10, -56)
	side:SetPoint("BOTTOMLEFT", 10, 10)
	side:SetWidth(SIDEBAR)
	backdrop(side, 0.13, 0.12, 0.11)
	side:EnableMouseWheel(true)
	side:SetScript("OnMouseWheel", function(_, delta)
		UI.scroll = math.max(0, UI.scroll - delta)
		UI.RenderSessions()
	end)

	local newBtn = button(side, ORANGE .. "+ New chat" .. RESET, SIDEBAR - 16, 26)
	newBtn:SetPoint("TOP", 0, -8)
	newBtn:SetScript("OnClick", function()
		WoWClaudeDB.current = ""
		UI.Render()
	end)

	f.rows = {}
	for i = 1, MAX_ROWS do
		local row = button(side, "", SIDEBAR - 16, ROW_HEIGHT - 2)
		row:SetPoint("TOP", newBtn, "BOTTOM", 0, -8 - (i - 1) * ROW_HEIGHT)
		row.text:ClearAllPoints()
		row.text:SetPoint("LEFT", 6, 0)
		row.text:SetPoint("RIGHT", -6, 0)
		row.text:SetJustifyH("LEFT")
		row:SetScript("OnClick", function(self)
			WoWClaudeDB.current = self.sessionID
			UI.Render()
		end)
		f.rows[i] = row
	end

	local scroll = CreateFrame("ScrollFrame", "WoWClaudeScroll", f, "UIPanelScrollFrameTemplate")
	scroll:SetPoint("TOPLEFT", side, "TOPRIGHT", 10, 0)
	scroll:SetPoint("BOTTOMRIGHT", -30, 48)
	f.scroll = scroll

	local log = CreateFrame("EditBox", nil, scroll)
	log:SetMultiLine(true)
	log:SetAutoFocus(false)
	log:SetFontObject(ChatFontNormal)
	log:SetWidth(scroll:GetWidth() > 0 and scroll:GetWidth() or 480)
	log:SetScript("OnEscapePressed", log.ClearFocus)
	log:SetScript("OnTextChanged", function(self, userInput)
		if userInput then
			self:SetText(self.fullText or "")
		end
	end)
	scroll:SetScrollChild(log)
	scroll:SetScript("OnSizeChanged", function(self, w) log:SetWidth(w) end)
	f.log = log

	local send = button(f, ORANGE .. "Send" .. RESET, 70, 26)
	send:SetPoint("BOTTOMRIGHT", -10, 10)
	send:SetScript("OnClick", function() UI.Send() end)

	local input = CreateFrame("EditBox", "WoWClaudeInput", f, "InputBoxTemplate")
	input:SetPoint("BOTTOMLEFT", side, "BOTTOMRIGHT", 16, 0)
	input:SetPoint("RIGHT", send, "LEFT", -8, 0)
	input:SetHeight(26)
	input:SetAutoFocus(false)
	input:SetMaxLetters(2000)
	input:SetScript("OnEnterPressed", function() UI.Send() end)
	input:SetScript("OnEscapePressed", input.ClearFocus)
	f.input = input
end

-- Send queues the prompt and reloads so the game flushes SavedVariables for the daemon.
function UI.Send()
	local text = strtrim(UI.frame.input:GetText())
	if text == "" then
		return
	end
	Core.Queue(WoWClaudeDB, WoWClaudeDB.current, text)
	UI.frame.input:SetText("")
	ReloadUI()
end

function UI.RenderSessions()
	local sessions = WoWClaudeInbox.sessions or {}
	local rows = UI.frame.rows
	UI.scroll = math.min(UI.scroll, math.max(0, #sessions - MAX_ROWS))
	for i = 1, MAX_ROWS do
		local s = sessions[i + UI.scroll]
		local row = rows[i]
		if s then
			row.sessionID = s.id
			local marker = (s.id == WoWClaudeDB.current) and ORANGE .. "▸ " .. RESET or "  "
			row.text:SetText(marker .. shorten(s.title ~= "" and s.title or s.id, 30))
			row:Show()
		else
			row:Hide()
		end
	end
end

function UI.RenderTranscript()
	local db, inbox = WoWClaudeDB, WoWClaudeInbox
	local lines = {}
	local current = db.current
	local s = Core.FindSession(inbox, current)
	if s then
		lines[#lines + 1] = DIM .. shorten(s.cwd, 70) .. RESET .. "\n"
	end
	if current == "" then
		lines[#lines + 1] = DIM .. "New chat in " .. (inbox.defaultCwd or "?") .. RESET .. "\n"
	end
	for _, m in ipairs(Core.Transcript(db, inbox, current)) do
		local who = m.role == "user" and ORANGE .. "You" .. RESET or CREAM .. "Claude" .. RESET
		if m.queued then
			who = who .. DIM .. "  (queued)" .. RESET
		end
		if m.pending then
			who = who .. DIM .. "  (working)" .. RESET
		end
		lines[#lines + 1] = who .. "\n" .. m.text .. "\n"
	end
	local text = table.concat(lines, "\n")
	UI.frame.log.fullText = text
	UI.frame.log:SetText(text)
	UI.frame.log:SetCursorPosition(#text)
end

function UI.Render()
	UI.scroll = UI.scroll or 0
	UI.frame.status:SetText(DIM .. Core.Status(WoWClaudeDB, WoWClaudeInbox) .. RESET)
	UI.RenderSessions()
	UI.RenderTranscript()
end

function UI.Toggle()
	if UI.frame:IsShown() then
		UI.frame:Hide()
		return
	end
	UI.Render()
	UI.frame:Show()
end

local loader = CreateFrame("Frame")
loader:RegisterEvent("ADDON_LOADED")
loader:SetScript("OnEvent", function(self, _, name)
	if name ~= addonName then
		return
	end
	self:UnregisterEvent("ADDON_LOADED")
	WoWClaudeDB = Core.InitDB(WoWClaudeDB)
	Core.Prune(WoWClaudeDB, WoWClaudeInbox.lastAckedID)
	Core.ResolveCurrent(WoWClaudeDB, WoWClaudeInbox)
	UI.Build()
	if WoWClaudeDB.open then
		UI.Render()
		UI.frame:Show()
	end
	UI.frame:HookScript("OnShow", function() WoWClaudeDB.open = true end)
	UI.frame:HookScript("OnHide", function() WoWClaudeDB.open = false end)
end)

SLASH_WOWCLAUDE1 = "/claude"
SLASH_WOWCLAUDE2 = "/wowclaude"
SlashCmdList.WOWCLAUDE = function(msg)
	local cmd, rest = msg:match("^(%S*)%s*(.-)$")
	if cmd == "cwd" then
		WoWClaudeDB.cwd = rest
		print(ORANGE .. "WoWClaude:" .. RESET .. " new chats will use cwd " .. (rest ~= "" and rest or "(daemon default)"))
		return
	end
	UI.Toggle()
end
