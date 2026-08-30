-- Runs on vanilla 1.12 (Lua 5.0, handlers read globals this/event/arg1) and Classic Era.
local ns = WoWClaudeNS
local Core = ns.Core

local ORANGE = "|cffd97757"
local CREAM = "|cffe8e3d8"
local DIM = "|cff8a857c"
local RESET = "|r"

local ROW_HEIGHT = 24
local SIDEBAR = 220
local MAX_ROWS = 15

-- Classic Era dropped table.getn; 1.12 has it and lacks the # operator.
local getn = table.getn or function(t) local n = 0 while t[n + 1] ~= nil do n = n + 1 end return n end
local strlen = string.len

-- BackdropTemplate exists only on modern clients; 1.12 frames carry SetBackdrop natively.
local BACKDROP = BackdropTemplateMixin and "BackdropTemplate" or nil

local UI = {}
ns.UI = UI

local function say(msg)
	DEFAULT_CHAT_FRAME:AddMessage(ORANGE .. "WoWClaude:" .. RESET .. " " .. msg)
end

local function trim(s)
	return (string.gsub(s or "", "^%s*(.-)%s*$", "%1"))
end

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
	local b = CreateFrame("Button", nil, parent, BACKDROP)
	b:SetWidth(width)
	b:SetHeight(height)
	backdrop(b, 0.22, 0.21, 0.2)
	b.text = b:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
	b.text:SetPoint("CENTER", b, "CENTER", 0, 0)
	b.text:SetText(text)
	b:SetScript("OnEnter", function() b:SetBackdropColor(0.3, 0.28, 0.26) end)
	b:SetScript("OnLeave", function() b:SetBackdropColor(0.22, 0.21, 0.2) end)
	return b
end

local function shorten(s, n)
	s = string.gsub(s or "", "\n", " ")
	if strlen(s) > n then
		return string.sub(s, 1, n - 3) .. "..."
	end
	return s
end

function UI.Build()
	local f = CreateFrame("Frame", "WoWClaudeFrame", UIParent, BACKDROP)
	UI.frame = f
	f:SetWidth(760)
	f:SetHeight(500)
	f:SetPoint("CENTER", UIParent, "CENTER", 0, 0)
	f:SetFrameStrata("DIALOG")
	backdrop(f, 0.16, 0.15, 0.14, 0.97)
	f:SetMovable(true)
	f:EnableMouse(true)
	f:RegisterForDrag("LeftButton")
	f:SetScript("OnDragStart", function() f:StartMoving() end)
	f:SetScript("OnDragStop", function() f:StopMovingOrSizing() end)
	f:SetScript("OnShow", function() WoWClaudeDB.open = true end)
	f:SetScript("OnHide", function() WoWClaudeDB.open = false end)
	if f.SetClampedToScreen then
		f:SetClampedToScreen(true)
	end
	f:Hide()
	tinsert(UISpecialFrames, "WoWClaudeFrame")

	local title = f:CreateFontString(nil, "OVERLAY", "GameFontNormalLarge")
	title:SetPoint("TOPLEFT", f, "TOPLEFT", 16, -12)
	title:SetText(ORANGE .. "* Claude" .. RESET)

	local close = button(f, "X", 24, 24)
	close:SetPoint("TOPRIGHT", f, "TOPRIGHT", -10, -10)
	close:SetScript("OnClick", function() f:Hide() end)

	local refresh = button(f, "Refresh", 70, 24)
	refresh:SetPoint("RIGHT", close, "LEFT", -6, 0)
	refresh:SetScript("OnClick", function() ReloadUI() end)

	f.status = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
	f.status:SetPoint("TOPLEFT", title, "BOTTOMLEFT", 0, -4)
	f.status:SetPoint("RIGHT", refresh, "LEFT", -10, 0)
	f.status:SetJustifyH("LEFT")

	local side = CreateFrame("Frame", nil, f, BACKDROP)
	side:SetPoint("TOPLEFT", f, "TOPLEFT", 10, -56)
	side:SetPoint("BOTTOMLEFT", f, "BOTTOMLEFT", 10, 10)
	side:SetWidth(SIDEBAR)
	backdrop(side, 0.13, 0.12, 0.11)
	side:EnableMouseWheel(true)
	side:SetScript("OnMouseWheel", function(_, delta)
		delta = delta or arg1
		UI.scroll = math.max(0, UI.scroll - delta)
		UI.RenderSessions()
	end)

	local newBtn = button(side, ORANGE .. "+ New chat" .. RESET, SIDEBAR - 16, 26)
	newBtn:SetPoint("TOP", side, "TOP", 0, -8)
	newBtn:SetScript("OnClick", function()
		WoWClaudeDB.current = ""
		UI.Render()
	end)

	f.rows = {}
	for i = 1, MAX_ROWS do
		local row = button(side, "", SIDEBAR - 16, ROW_HEIGHT - 2)
		row:SetPoint("TOP", newBtn, "BOTTOM", 0, -8 - (i - 1) * ROW_HEIGHT)
		row.text:ClearAllPoints()
		row.text:SetPoint("LEFT", row, "LEFT", 6, 0)
		row.text:SetPoint("RIGHT", row, "RIGHT", -6, 0)
		row.text:SetJustifyH("LEFT")
		row:SetScript("OnClick", function()
			WoWClaudeDB.current = row.sessionID
			UI.Render()
		end)
		f.rows[i] = row
	end

	local scroll = CreateFrame("ScrollFrame", "WoWClaudeScroll", f, "UIPanelScrollFrameTemplate")
	scroll:SetPoint("TOPLEFT", side, "TOPRIGHT", 10, 0)
	scroll:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -30, 48)
	f.scroll = scroll

	local log = CreateFrame("EditBox", "WoWClaudeLog", scroll)
	log:SetMultiLine(true)
	log:SetAutoFocus(false)
	log:SetFontObject(ChatFontNormal)
	log:SetWidth(480)
	log:SetHeight(400)
	log:SetScript("OnEscapePressed", function() log:ClearFocus() end)
	-- Read-only: any edit is reverted; comparing first avoids a SetText/OnTextChanged loop on 1.12.
	log:SetScript("OnTextChanged", function()
		local want = log.fullText or ""
		if log:GetText() ~= want then
			log:SetText(want)
		end
	end)
	scroll:SetScrollChild(log)
	scroll:SetScript("OnSizeChanged", function() log:SetWidth(scroll:GetWidth()) end)
	f.log = log

	local send = button(f, ORANGE .. "Send" .. RESET, 70, 26)
	send:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -10, 10)
	send:SetScript("OnClick", function() UI.Send() end)

	local input = CreateFrame("EditBox", "WoWClaudeInput", f, "InputBoxTemplate")
	input:SetPoint("BOTTOMLEFT", side, "BOTTOMRIGHT", 16, 0)
	input:SetPoint("RIGHT", send, "LEFT", -8, 0)
	input:SetHeight(26)
	input:SetAutoFocus(false)
	input:SetMaxLetters(2000)
	input:SetScript("OnEnterPressed", function() UI.Send() end)
	input:SetScript("OnEscapePressed", function() input:ClearFocus() end)
	f.input = input
end

-- Send queues the prompt and reloads so the game flushes SavedVariables for the daemon.
function UI.Send()
	local text = trim(UI.frame.input:GetText())
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
	UI.scroll = math.min(UI.scroll, math.max(0, getn(sessions) - MAX_ROWS))
	for i = 1, MAX_ROWS do
		local s = sessions[i + UI.scroll]
		local row = rows[i]
		if s then
			row.sessionID = s.id
			local marker = "  "
			if s.id == WoWClaudeDB.current then
				marker = ORANGE .. "> " .. RESET
			end
			local label = s.title
			if label == "" then
				label = s.id
			end
			row.text:SetText(marker .. shorten(label, 30))
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
		table.insert(lines, DIM .. shorten(s.cwd, 70) .. RESET .. "\n")
	end
	if current == "" then
		table.insert(lines, DIM .. "New chat in " .. (inbox.defaultCwd or "?") .. RESET .. "\n")
	end
	for _, m in ipairs(Core.Transcript(db, inbox, current)) do
		local who = CREAM .. "Claude" .. RESET
		if m.role == "user" then
			who = ORANGE .. "You" .. RESET
		end
		if m.queued then
			who = who .. DIM .. "  (queued)" .. RESET
		end
		if m.pending then
			who = who .. DIM .. "  (working)" .. RESET
		end
		table.insert(lines, who .. "\n" .. m.text .. "\n")
	end
	local text = table.concat(lines, "\n")
	local log = UI.frame.log
	log.fullText = text
	log:SetText(text)
	if log.SetCursorPosition then
		log:SetCursorPosition(strlen(text))
	end
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
loader:SetScript("OnEvent", function(_, _, name)
	name = name or arg1
	if name ~= "WoWClaude" then
		return
	end
	loader:UnregisterEvent("ADDON_LOADED")
	WoWClaudeDB = Core.InitDB(WoWClaudeDB)
	Core.Prune(WoWClaudeDB, WoWClaudeInbox.lastAckedID, WoWClaudeInbox.epoch)
	Core.ResolveCurrent(WoWClaudeDB, WoWClaudeInbox)
	local reopen = WoWClaudeDB.open
	UI.Build()
	if reopen then
		UI.Render()
		UI.frame:Show()
	end
end)

SLASH_WOWCLAUDE1 = "/claude"
SLASH_WOWCLAUDE2 = "/wowclaude"
SlashCmdList.WOWCLAUDE = function(msg)
	local _, _, cmd, rest = string.find(msg or "", "^(%S*)%s*(.-)$")
	if cmd == "cwd" then
		WoWClaudeDB.cwd = rest
		if rest == "" then
			rest = "(daemon default)"
		end
		say("new chats will use cwd " .. rest)
		return
	end
	UI.Toggle()
end
