-- states
-- 0 - idle
-- 1 - wait for sync = have request to send
-- 2 - expect my master id
-- 3 - wait for response
-- frame: S. D. C.. L. Dx CRC. | ACKs. L. Dx CRC. ACKm. SYN.   

state = 0
pos = 0
request = ""
frame = ""

function analyze_frame (conn, frm)
	reql = request:len()
	if frm:len() < reql + 2 then
		-- still waiting for data (request, acl, response len)
		return 0
	end
	if frm:sub(1, reql) ~= request then
		-- request collision
		return 1
	end
	if string.byte(frm:sub(reql+1, reql+1)) ~= 0x00 then
		-- recipient did not ACK
		return 2
	end
	respl = string.byte(frm:sub(reql+2))
	expl = reql + 2 + respl + 1
	if frm:len() < expl then
		-- still waiting for data (request, acl, response)
		return 0
	end
	-- TODO: verify CRC
	uart.write(0, string.char(0x00))

	uart.write(0, string.char(0xaa))
	return 3
end

tmr.alarm(0,200,0,function() -- run after a delay

    uart.setup(0, 2400, 8, 0, 1, 1)

    srv=net.createServer(net.TCP, 28800) 
    srv:listen(3333,function(conn)
     
        uart.on("data", 0, function(data)
            conn:send(data)
            if state == 1 then
            	if data:len() == 1 and string.byte(data) == 0xaa then
            		uart.write(0, request:sub(1,1))
            		state = 2
            	end
        	elseif state == 2 then
        		if data:len() == 1 and string.byte(data) == string.byte(request) then
        			uart.write(0, request:sub(2))
        			frame = data
        			state = 3
        		else
        			state = 1
        		end
        	elseif state == 3 then
        		frame = frame..data
        		res = analyze_frame(conn, frame)
        		if res > 0 then
        			state = 0
        		end
        	end
        end, 0)
        
        conn:on("receive",function(conn,payload) 
            request = payload
            state = 1
        end)  
        
        conn:on("disconnection",function(c) 
            uart.on("data")
        end)
        
    end)
end)
