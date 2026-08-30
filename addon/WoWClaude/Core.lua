-- Lua 5.0 (vanilla 1.12) and 5.1 (Classic Era): no "#", no varargs namespace, no string.match.
WoWClaudeNS = WoWClaudeNS or {}
local ns = WoWClaudeNS

-- Pure state logic, no frames, so it runs under plain Lua in tests.
local Core = {}
ns.Core = Core

-- Classic Era dropped table.getn; 1.12 has it and lacks the # operator.
local getn = table.getn or function(t) local n = 0 while t[n + 1] ~= nil do n = n + 1 end return n end

function Core.InitDB(db)
	db = db or {}
	db.seq = db.seq or 0
	db.outbox = db.outbox or {}
	db.current = db.current or ""
	db.awaitingNew = db.awaitingNew or false
	db.epoch = db.epoch or Core.NewEpoch()
	return db
end

function Core.Now()
	return (time or os.time)()
end

-- SignalEpoch mirrors the daemon: a dot in a texture path reads as an extension.
function Core.SignalEpoch(epoch)
	return (string.gsub(epoch or "", "%.", "_"))
end

-- NewEpoch stamps a fresh SavedVariables so the daemon can tell a reset seq from an old one.
function Core.NewEpoch()
	local now = (time or os.time)()
	return tostring(now) .. "-" .. tostring(math.random(1, 999999))
end

-- Prune drops outbox entries the daemon has already picked up; an ack from another epoch is stale.
function Core.Prune(db, lastAckedID, epoch)
	if epoch ~= db.epoch then
		return
	end
	local kept = {}
	for _, e in ipairs(db.outbox) do
		if e.id > (lastAckedID or 0) then
			table.insert(kept, e)
		end
	end
	db.outbox = kept
end

function Core.Queue(db, sessionID, prompt)
	db.seq = db.seq + 1
	local entry = { id = db.seq, session = sessionID or "", cwd = db.cwd or "", prompt = prompt }
	table.insert(db.outbox, entry)
	if entry.session == "" then
		db.awaitingNew = prompt
	end
	entry.slot = db.slotBase or 0
	db.waitFor = entry.id
	db.waitSlot = entry.slot
	db.sentAt = Core.Now()
	return entry
end

-- FirstFreeSlot finds the lowest reply slot not yet loaded this game session; 0 means none left.
function Core.FirstFreeSlot(isLoaded, n)
	for i = 1, n do
		if not isLoaded(Core.SlotName(i)) then
			return i
		end
	end
	return 0
end

function Core.SlotName(slot)
	return string.format("WoWClaudeIn%04d", slot)
end

-- ApplyReply merges a live reply into the inbox; a reply for another turn or epoch is ignored.
function Core.ApplyReply(db, inbox, reply)
	if not reply or reply.epoch ~= db.epoch or reply.id ~= db.waitFor then
		return false
	end
	inbox.error = reply.error or ""
	inbox.active = nil
	inbox.lastDoneID = reply.id
	local s = reply.session
	if s then
		local kept = { s }
		for _, old in ipairs(inbox.sessions or {}) do
			if old.id ~= s.id then
				table.insert(kept, old)
			end
		end
		inbox.sessions = kept
		db.current = s.id
		db.awaitingNew = false
	end
	Core.Prune(db, reply.id, reply.epoch)
	db.waitFor = nil
	db.sentAt = nil
	return true
end

-- Settle clears the wait once the daemon reports the turn done under the same epoch.
function Core.Settle(db, inbox)
	if not db.waitFor then
		return false
	end
	if inbox.epoch ~= db.epoch or (inbox.lastDoneID or 0) < db.waitFor then
		return false
	end
	db.waitFor = nil
	db.sentAt = nil
	return true
end

function Core.FindSession(inbox, id)
	for _, s in ipairs(inbox.sessions or {}) do
		if s.id == id then
			return s
		end
	end
	return nil
end

-- ResolveCurrent jumps to the session a "new chat" prompt created, matched by its first turn.
function Core.ResolveCurrent(db, inbox)
	if db.current ~= "" and not Core.FindSession(inbox, db.current) then
		db.current = ""
	end
	if not db.awaitingNew then
		return
	end
	if getn(db.outbox) > 0 or (inbox.active and inbox.active.sessionID == "") then
		return
	end
	local prompt = db.awaitingNew
	db.awaitingNew = false
	for _, s in ipairs(inbox.sessions or {}) do
		local first = s.messages and s.messages[1]
		if first and first.text == prompt then
			db.current = s.id
			return
		end
	end
end

function Core.Status(db, inbox)
	if inbox.error and inbox.error ~= "" then
		return "error: " .. inbox.error
	end
	if getn(db.outbox) > 0 then
		return getn(db.outbox) .. " queued, waiting for the daemon (press Refresh)"
	end
	if inbox.active or db.waitFor then
		return "Claude is thinking"
	end
	return "idle, inbox from " .. (inbox.generatedAt or "?")
end

-- Transcript merges saved turns, the running turn, and queued prompts for one session.
function Core.Transcript(db, inbox, sessionID)
	local out = {}
	local s = Core.FindSession(inbox, sessionID)
	if s then
		for _, m in ipairs(s.messages or {}) do
			table.insert(out, { role = m.role, text = m.text })
		end
	end
	local a = inbox.active
	if a and a.sessionID == sessionID then
		table.insert(out, { role = "user", text = a.prompt })
		local partial = a.partial or ""
		if partial == "" then
			partial = "..."
		end
		table.insert(out, { role = "assistant", text = partial, pending = true })
	end
	for _, e in ipairs(db.outbox) do
		if e.session == sessionID then
			table.insert(out, { role = "user", text = e.prompt, queued = true })
		end
	end
	return out
end
