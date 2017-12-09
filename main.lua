uart.on("data", 0, function(data) end, 0);

tmr.create():alarm(60000, tmr.ALARM_SINGLE, function()
	dofile("program.lua")
end)

