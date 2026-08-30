-- Runs on vanilla 1.12 (Lua 5.0, handlers read globals this/event/arg1) and Classic Era.
local ns = WoWClaudeNS
local Core = ns.Core

local GOLD = "|cffffd100"
local WHITE = "|cffffffff"
local DIM = "|cff9d9d9d"
local RESET = "|r"

local ROW_HEIGHT = 20
local SIDEBAR = 200
local MAX_ROWS = 16

-- Classic Era dropped table.getn; 1.12 has it and lacks the # operator.
local getn = table.getn or function(t) local n = 0 while t[n + 1] ~= nil do n = n + 1 end return n end
local strlen = string.len

-- BackdropTemplate exists only on modern clients; 1.12 frames carry SetBackdrop natively.
local BACKDROP = BackdropTemplateMixin and "BackdropTemplate" or nil

local UI = {}
ns.UI = UI

local function say(msg)
	DEFAULT_CHAT_FRAME:AddMessage(GOLD .. "WoWClaude:" .. RESET .. " " .. msg)
end

local function trim(s)
	return (string.gsub(s or "", "^%s*(.-)%s*$", "%1"))
end

-- The stock dialog skin: the same art as the game's own popups and the Blizzard options panels.
local function dialogSkin(frame)
	frame:SetBackdrop({
		bgFile = "Interface\\DialogFrame\\UI-DialogBox-Background",
		edgeFile = "Interface\\DialogFrame\\UI-DialogBox-Border",
		tile = true, tileSize = 32, edgeSize = 32,
		insets = { left = 11, right = 12, top = 12, bottom = 11 },
	})
end

-- The tooltip skin, which addons use for inset panels and lists.
local function insetSkin(frame)
	frame:SetBackdrop({
		bgFile = "Interface\\Tooltips\\UI-Tooltip-Background",
		edgeFile = "Interface\\Tooltips\\UI-Tooltip-Border",
		tile = true, tileSize = 16, edgeSize = 16,
		insets = { left = 4, right = 4, top = 4, bottom = 4 },
	})
	frame:SetBackdropColor(0, 0, 0, 0.75)
	frame:SetBackdropBorderColor(0.6, 0.6, 0.6, 1)
end

local function button(parent, text, width)
	local b = CreateFrame("Button", nil, parent, "UIPanelButtonTemplate")
	b:SetWidth(width)
	b:SetHeight(22)
	b:SetText(text)
	return b
end

-- A quest-log style list row: plain text with the gold hover glow.
local function row(parent, width)
	local b = CreateFrame("Button", nil, parent)
	b:SetWidth(width)
	b:SetHeight(ROW_HEIGHT)
	b:SetHighlightTexture("Interface\\QuestFrame\\UI-QuestTitleHighlight", "ADD")
	b.selected = b:CreateTexture(nil, "BACKGROUND")
	b.selected:SetTexture("Interface\\QuestFrame\\UI-QuestTitleHighlight")
	b.selected:SetBlendMode("ADD")
	b.selected:SetAllPoints(b)
	b.selected:Hide()
	b.text = b:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
	b.text:SetPoint("LEFT", b, "LEFT", 6, 0)
	b.text:SetPoint("RIGHT", b, "RIGHT", -6, 0)
	b.text:SetJustifyH("LEFT")
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
	dialogSkin(f)
	f:SetMovable(true)
	f:EnableMouse(true)
	f:RegisterForDrag("LeftButton")
	f:SetScript("OnDragStart", function() f:StartMoving() end)
	f:SetScript("OnDragStop", function() f:StopMovingOrSizing() end)
	f:SetScript("OnUpdate", function() UI.Tick() end)
	if f.SetClampedToScreen then
		f:SetClampedToScreen(true)
	end
	f:Hide()
	tinsert(UISpecialFrames, "WoWClaudeFrame")

	local header = f:CreateTexture(nil, "ARTWORK")
	header:SetTexture("Interface\\DialogFrame\\UI-DialogBox-Header")
	header:SetWidth(256)
	header:SetHeight(64)
	header:SetPoint("TOP", f, "TOP", 0, 12)

	local title = f:CreateFontString(nil, "OVERLAY", "GameFontNormal")
	title:SetPoint("TOP", header, "TOP", 0, -14)
	title:SetText("Claude")

	local close = CreateFrame("Button", nil, f, "UIPanelCloseButton")
	close:SetPoint("TOPRIGHT", f, "TOPRIGHT", -6, -6)
	close:SetScript("OnClick", function() UI.Toggle() end)

	local refresh = button(f, "Refresh", 80)
	refresh:SetPoint("TOPRIGHT", f, "TOPRIGHT", -36, -14)
	refresh:SetScript("OnClick", function() ReloadUI() end)

	f.status = f:CreateFontString(nil, "OVERLAY", "GameFontHighlightSmall")
	f.status:SetPoint("TOPLEFT", f, "TOPLEFT", 20, -40)
	f.status:SetPoint("RIGHT", refresh, "LEFT", -10, 0)
	f.status:SetJustifyH("LEFT")

	local side = CreateFrame("Frame", nil, f, BACKDROP)
	side:SetPoint("TOPLEFT", f, "TOPLEFT", 16, -56)
	side:SetPoint("BOTTOMLEFT", f, "BOTTOMLEFT", 16, 16)
	side:SetWidth(SIDEBAR)
	insetSkin(side)
	side:EnableMouseWheel(true)
	side:SetScript("OnMouseWheel", function(_, delta)
		delta = delta or arg1
		UI.scroll = math.max(0, UI.scroll - delta)
		UI.RenderSessions()
	end)

	local newBtn = button(side, "New Chat", SIDEBAR - 16)
	newBtn:SetPoint("TOP", side, "TOP", 0, -8)
	newBtn:SetScript("OnClick", function()
		WoWClaudeDB.current = ""
		UI.Render()
	end)

	f.rows = {}
	for i = 1, MAX_ROWS do
		local r = row(side, SIDEBAR - 16)
		r:SetPoint("TOP", newBtn, "BOTTOM", 0, -6 - (i - 1) * ROW_HEIGHT)
		r:SetScript("OnClick", function()
			WoWClaudeDB.current = r.sessionID
			UI.Render()
		end)
		f.rows[i] = r
	end

	local pane = CreateFrame("Frame", nil, f, BACKDROP)
	pane:SetPoint("TOPLEFT", side, "TOPRIGHT", 8, 0)
	pane:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -16, 50)
	insetSkin(pane)

	local scroll = CreateFrame("ScrollFrame", "WoWClaudeScroll", pane, "UIPanelScrollFrameTemplate")
	scroll:SetPoint("TOPLEFT", pane, "TOPLEFT", 8, -8)
	scroll:SetPoint("BOTTOMRIGHT", pane, "BOTTOMRIGHT", -28, 8)
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

	-- A hidden texture probes for the daemon's signal file; SetTexture reports whether a file exists.
	f.probe = f:CreateTexture(nil, "BACKGROUND")
	f.probe:Hide()
	-- Self-test: a client that "finds" a missing file cannot poll, so it falls back to Refresh.
	UI.canProbe = not UI.SignalExists("never")

	local send = button(f, "Send", 80)
	send:SetPoint("BOTTOMRIGHT", f, "BOTTOMRIGHT", -16, 16)
	send:SetScript("OnClick", function() UI.Send() end)

	local input = CreateFrame("EditBox", "WoWClaudeInput", f, "InputBoxTemplate")
	input:SetPoint("BOTTOMLEFT", side, "BOTTOMRIGHT", 16, 0)
	input:SetPoint("RIGHT", send, "LEFT", -8, 0)
	input:SetHeight(24)
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
	UI.Reload()
end

-- Reload flushes SavedVariables for the daemon; the flag reopens the window on the other side.
function UI.Reload()
	WoWClaudeDB.open = true
	ReloadUI()
end

function UI.SignalExists(name)
	local path = "Interface\\AddOns\\WoWClaude\\signal\\" .. Core.SignalEpoch(WoWClaudeDB.epoch) .. "\\" .. name
	local probe = UI.frame.probe
	local ok = probe:SetTexture(path)
	if ok == nil then
		ok = probe:GetTexture()
	end
	probe:SetTexture(nil)
	return ok and ok ~= 0 and ok ~= "" and true or false
end

-- Tick animates the wait and reloads once, the moment the daemon signals the turn is done.
function UI.Tick()
	local db = WoWClaudeDB
	if not db.waitFor then
		return
	end
	local now = GetTime()
	if UI.lastTick and now - UI.lastTick < 0.25 then
		return
	end
	UI.lastTick = now
	local n = math.floor(now * 2)
	local dots = string.rep(".", 1 + n - math.floor(n / 3) * 3)
	local elapsed = Core.Now() - (db.sentAt or Core.Now())
	UI.frame.status:SetText(GOLD .. "Claude is thinking" .. dots .. RESET .. DIM .. "  " .. elapsed .. "s" .. RESET)
	if not UI.canProbe or db.autoReloadedFor == db.waitFor then
		return
	end
	if UI.lastProbe and now - UI.lastProbe < 2 then
		return
	end
	UI.lastProbe = now
	if UI.SignalExists(tostring(db.waitFor)) then
		db.autoReloadedFor = db.waitFor
		UI.Reload()
	end
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
			if s.id == WoWClaudeDB.current then
				row.selected:Show()
			end
			if s.id ~= WoWClaudeDB.current then
				row.selected:Hide()
			end
			local label = s.title
			if label == "" then
				label = s.id
			end
			row.text:SetText(shorten(label, 28))
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
		local who = WHITE .. "Claude" .. RESET
		if m.role == "user" then
			who = GOLD .. "You" .. RESET
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
		WoWClaudeDB.open = false
		UI.frame:Hide()
		return
	end
	WoWClaudeDB.open = true
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
	Core.Settle(WoWClaudeDB, WoWClaudeInbox)
	UI.Build()
	if WoWClaudeDB.open then
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
