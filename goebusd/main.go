package main

import (
	"encoding/binary"
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
	"github.com/gorilla/mux"
)

type Frame struct {
	data	[]byte
	ts	time.Time
	next	*Frame
}

type Metric struct {
	name	string
	id	[]byte
	format	string
}

type Request struct {
	req	[]byte
	start	*Frame
	res	[]byte
	crcOk	bool
	finished bool
}

func (r *Request) Response() []byte {
	for !r.finished {
		time.Sleep(100*time.Millisecond)
	}
	return r.res
}

var send_queue chan []byte
var cur_frame *Frame
var config map[string]Metric

var crc_table = []byte{0x00, 0x9b, 0xad, 0x36, 0xc1, 0x5a, 0x6c, 0xf7, 0x19, 0x82, 0xb4, 0x2f, 0xd8, 0x43, 0x75, 0xee,
	0x32, 0xa9, 0x9f, 0x04, 0xf3, 0x68, 0x5e, 0xc5, 0x2b, 0xb0, 0x86, 0x1d, 0xea, 0x71, 0x47, 0xdc,
	0x64, 0xff, 0xc9, 0x52, 0xa5, 0x3e, 0x08, 0x93, 0x7d, 0xe6, 0xd0, 0x4b, 0xbc, 0x27, 0x11, 0x8a,
	0x56, 0xcd, 0xfb, 0x60, 0x97, 0x0c, 0x3a, 0xa1, 0x4f, 0xd4, 0xe2, 0x79, 0x8e, 0x15, 0x23, 0xb8,
	0xc8, 0x53, 0x65, 0xfe, 0x09, 0x92, 0xa4, 0x3f, 0xd1, 0x4a, 0x7c, 0xe7, 0x10, 0x8b, 0xbd, 0x26,
	0xfa, 0x61, 0x57, 0xcc, 0x3b, 0xa0, 0x96, 0x0d, 0xe3, 0x78, 0x4e, 0xd5, 0x22, 0xb9, 0x8f, 0x14,
	0xac, 0x37, 0x01, 0x9a, 0x6d, 0xf6, 0xc0, 0x5b, 0xb5, 0x2e, 0x18, 0x83, 0x74, 0xef, 0xd9, 0x42,
	0x9e, 0x05, 0x33, 0xa8, 0x5f, 0xc4, 0xf2, 0x69, 0x87, 0x1c, 0x2a, 0xb1, 0x46, 0xdd, 0xeb, 0x70,
	0x0b, 0x90, 0xa6, 0x3d, 0xca, 0x51, 0x67, 0xfc, 0x12, 0x89, 0xbf, 0x24, 0xd3, 0x48, 0x7e, 0xe5,
	0x39, 0xa2, 0x94, 0x0f, 0xf8, 0x63, 0x55, 0xce, 0x20, 0xbb, 0x8d, 0x16, 0xe1, 0x7a, 0x4c, 0xd7,
	0x6f, 0xf4, 0xc2, 0x59, 0xae, 0x35, 0x03, 0x98, 0x76, 0xed, 0xdb, 0x40, 0xb7, 0x2c, 0x1a, 0x81,
	0x5d, 0xc6, 0xf0, 0x6b, 0x9c, 0x07, 0x31, 0xaa, 0x44, 0xdf, 0xe9, 0x72, 0x85, 0x1e, 0x28, 0xb3,
	0xc3, 0x58, 0x6e, 0xf5, 0x02, 0x99, 0xaf, 0x34, 0xda, 0x41, 0x77, 0xec, 0x1b, 0x80, 0xb6, 0x2d,
	0xf1, 0x6a, 0x5c, 0xc7, 0x30, 0xab, 0x9d, 0x06, 0xe8, 0x73, 0x45, 0xde, 0x29, 0xb2, 0x84, 0x1f,
	0xa7, 0x3c, 0x0a, 0x91, 0x66, 0xfd, 0xcb, 0x50, 0xbe, 0x25, 0x13, 0x88, 0x7f, 0xe4, 0xd2, 0x49,
	0x95, 0x0e, 0x38, 0xa3, 0x54, 0xcf, 0xf9, 0x62, 0x8c, 0x17, 0x21, 0xba, 0x4d, 0xd6, 0xe0, 0x7b}

func calc_crc(data []byte) byte {
	crc := byte(0)
	for _, ch := range data {
		crc = crc_table[crc] ^ ch
	}
	log.Printf("crc: %x %x",data, crc)
	return crc
}

func read_frame(reader *bufio.Reader) []byte {
	var B byte
	var frame []byte
	for B != 0xaa {
		B,_ = reader.ReadByte()
		frame = append(frame, B)
	}
	return frame
}

func handle_conn(){
	conn, err := net.Dial("tcp", "192.168.0.115:3333")
	if err != nil {
		log.Panic(err)
	}

	//conn.SetReadDeadline(time.Time(0))
	reader := bufio.NewReader(conn)

	for {

		frame := read_frame(reader)

		select {
		case request := <-send_queue:
			log.Printf("send  %x", request)
			conn.Write(request)
		default:
		}

		if len(frame) > 1 {
			log.Printf("frame %x", frame)
			cur_frame.next = &Frame{data: frame, ts: time.Now()}
			cur_frame=cur_frame.next
		}
	}
}

func get_frame_match(start *Frame, data []byte) *Frame {
	//TODO: proper search for response
	searchLimit := 5
	i := 0
	cur := get_next_frame(start)
	for ! bytes.HasPrefix(cur.data, data) {
		cur = get_next_frame(cur)
		i++
		if i > searchLimit {
			return nil
		}
	}
	return cur
}

func get_next_frame(frame *Frame) *Frame {
	for frame.next == nil {
		time.Sleep(10*time.Millisecond)
	}
	return frame.next
}

func find_response(req *Request) {
	rqlen := len(req.req)
	defer func() { req.finished = true }()

	resp_frame := get_frame_match(req.start, req.req)
	if resp_frame == nil {
		log.Print("no frame found")
		return
	}

	frame := resp_frame.data

	if len(frame) < rqlen + 3 {
		log.Print("frame too short")
		return
	}

	if frame[rqlen-1] != 0x00 {
		log.Print("request NACK-ed")
		return
	}

	rest := frame[rqlen+2:]
	rtlen := len(rest)
	rslen := int(rest[0])
	if rtlen < rslen + 1 {
		log.Print("frame too short")
		return
	}

	resp := rest[:rslen+1]
	req.res = resp
	if rest[rslen+1] == calc_crc(resp) {
		req.crcOk = true
	} else {
		log.Printf("CRC INVALID!!!")
	}
	log.Printf("resp %x", resp)
	return
}

func request_raw(request []byte) *Request {
	req := &Request {
		start: cur_frame,
		req: request,
	}
	request = append(request, calc_crc(request))
	go find_response(req)
	send_queue <- request
	return req
}

func request_metric(metric Metric) *Request {
	log.Print("requesting %s", metric)
	req_bytes := append([]byte {0x31, 0x08, 0xb5, 0x09, 0x03, 0x0d }, metric.id...)
	req := request_raw(req_bytes)
	return req
}

func parse_response(data []byte, format string) string {
	buf := bytes.NewReader(data[1:])
	var UInt16 uint16
	switch format {
	case "onoff":
		if data[1] == 0 {
			return "0"
		} else {
			return "1"
		}
	case "tempmirrorsensor":
		fallthrough
	case "tempsensor":
		fallthrough
	case "temp":
		binary.Read(buf, binary.LittleEndian, &UInt16)
		return fmt.Sprintf("%f", float32(UInt16)/16)
	}
	return fmt.Sprintf("unknown format: %x", data)
}

func handle_raw(w http.ResponseWriter, r *http.Request) {
	log.Print("http request")
	request := []byte{0x31, 0x08, 0xb5, 0x09, 0x03, 0x0d, 0x0E, 0x00}
	w.Write(request_raw(request).Response())
}

func handle_get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metric := config[vars["metric"]]
	req := request_metric(metric)
	w.Write([]byte(parse_response(req.Response(), metric.format)+"\n"))
}

func load_config(file string){
	csvFile, _ := os.Open(file)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	config = make(map[string]Metric)
	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Print(error)
			continue
		}
		hex_id, _ := hex.DecodeString(line[7])
		config[line[2]] = Metric{ id: hex_id, format: line[10] }
	}

	log.Print(config)

}

func update_loop() {
	for _,metric := range config {
		request_metric(metric)
		time.Sleep(100*time.Millisecond)
	}
	time.Sleep(30*time.Second)
}

func main() {
	load_config("../bai.0010015600.inc")
	send_queue = make(chan []byte, 5)
	cur_frame = &Frame{}
	go handle_conn()

	//go update_loop()

	r := mux.NewRouter()
	// Routes consist of a path and a handler function.
	r.HandleFunc("/raw", handle_raw)
	r.HandleFunc("/get/{metric}", handle_get)

	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":8085", r))

}
