#!/usr/bin/env python

import socket


TCP_IP = '192.168.0.115'
TCP_PORT = 3333
BUFFER_SIZE = 1024
MESSAGE = "Hello, World!"

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.connect((TCP_IP, TCP_PORT))
#s.send(MESSAGE)

chold = "a"
while True:
  ch = s.recv(1)
  if ch != chr(0xaa):
    if chold == chr(0xaa):
      print "\n"
    print ch.encode("hex"),
  chold = ch

s.close()

