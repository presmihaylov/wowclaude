-- Minimal test runner: luajit addon/tests/run.lua
local here = arg[0]:match("^(.*)/[^/]+$") or "."
local addonDir = here .. "/../WoWClaude/"

local function load(file)
	local chunk, err = loadfile(addonDir .. file)
	if not chunk then error(err) end
	return chunk()
end

load("Core.lua")
local Core = WoWClaudeNS.Core

local tests, failed = {}, 0
local function test(name, fn) tests[#tests + 1] = { name = name, fn = fn } end
local function eq(got, want, label)
	if got ~= want then
		error(string.format("%s: got %s, want %s", label or "value", tostring(got), tostring(want)), 2)
	end
end

test("Queue assigns ids and flags new chats", function()
	local db = Core.InitDB(nil)
	local e1 = Core.Queue(db, "", "hello")
	local e2 = Core.Queue(db, "abc", "again")
	eq(e1.id, 1); eq(e2.id, 2); eq(table.getn(db.outbox), 2)
	eq(db.awaitingNew, "hello")
	eq(e2.session, "abc")
end)

test("Prune drops acked entries", function()
	local db = Core.InitDB(nil)
	Core.Queue(db, "", "a"); Core.Queue(db, "", "b"); Core.Queue(db, "", "c")
	Core.Prune(db, 2)
	eq(table.getn(db.outbox), 1); eq(db.outbox[1].prompt, "c")
end)

test("ResolveCurrent finds the session a new chat created", function()
	local db = Core.InitDB(nil)
	Core.Queue(db, "", "start")
	local inbox = { lastAckedID = 1, active = { sessionID = "", prompt = "start", partial = "" }, sessions = {} }
	Core.Prune(db, inbox.lastAckedID)
	Core.ResolveCurrent(db, inbox)
	eq(db.current, "", "still running")
	inbox.active = nil
	inbox.sessions = {
		{ id = "busy", messages = { { role = "user", text = "unrelated" } } },
		{ id = "new-1", messages = { { role = "user", text = "start" }, { role = "assistant", text = "ok" } } },
	}
	Core.ResolveCurrent(db, inbox)
	eq(db.current, "new-1"); eq(db.awaitingNew, false)
end)

test("ResolveCurrent clears a session that vanished", function()
	local db = Core.InitDB({ current = "gone" })
	Core.ResolveCurrent(db, { sessions = {} })
	eq(db.current, "")
end)

test("Transcript merges saved, active and queued turns", function()
	local db = Core.InitDB(nil)
	Core.Queue(db, "s1", "third?")
	local inbox = {
		sessions = { { id = "s1", messages = { { role = "user", text = "first" }, { role = "assistant", text = "reply" } } } },
		active = { sessionID = "s1", prompt = "second?", partial = "" },
	}
	local t = Core.Transcript(db, inbox, "s1")
	eq(table.getn(t), 5)
	eq(t[3].text, "second?"); eq(t[4].text, "..."); eq(t[4].pending, true)
	eq(t[5].text, "third?"); eq(t[5].queued, true)
	eq(table.getn(Core.Transcript(db, inbox, "other")), 0)
end)

test("Status prefers error, then queue, then active", function()
	local db = Core.InitDB(nil)
	eq(Core.Status(db, { error = "boom" }):sub(1, 6), "error:")
	Core.Queue(db, "", "x")
	eq(Core.Status(db, { error = "" }):sub(1, 8), "1 queued")
	db.outbox = {}
	eq(Core.Status(db, { active = {} }):sub(1, 6), "Claude")
	eq(Core.Status(db, { generatedAt = "t" }), "idle, inbox from t")
end)

for _, t in ipairs(tests) do
	local ok, err = pcall(t.fn)
	if ok then
		print("ok   " .. t.name)
	else
		failed = failed + 1
		print("FAIL " .. t.name .. "\n     " .. tostring(err))
	end
end
print(string.format("%d tests, %d failed", #tests, failed))
os.exit(failed == 0 and 0 or 1)
