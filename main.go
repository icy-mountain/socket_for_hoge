package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	str "strings"
	"time"
)

//wait: Server waits wait-second until client's timeout.
//pass_points: Server pass client if client solves pass_points ploblems.
const (
	wait        = 3
	pass_points = 1
)

//incoming: Server -> incoming -> Client
//outgoing: Server <- outgoing <- Client
//ticker: To manage timeout 
type Client struct {
	idx      int
	question string
	timeout  bool
	corrects int
	incoming chan string
	outgoing chan string
	conn     net.Conn
	ticker   *time.Ticker
	reader   *bufio.Reader
	writer   *bufio.Writer
}

//incoming: Server <- incoming <- Client
//outgoing: Server -> outgoing -> Client
type Server struct {
	conn     chan net.Conn
	incoming chan string
	outgoing chan string
	listener *net.TCPListener
	clients  []*Client
}

//return new client to server
func newClient(connection net.Conn, length int) *Client {
	writer := bufio.NewWriter(connection)
	reader := bufio.NewReader(connection)
	ticker := time.NewTicker(wait * time.Second)

	client := &Client{
		idx:      length,
		question: "",
		timeout:  true,
		corrects: 0,
		incoming: make(chan string),
		outgoing: make(chan string, 20),
		conn:     connection,
		ticker:   ticker,
		reader:   reader,
		writer:   writer,
	}

	//start sleeping api server 
	_ = get_flag()
	go client.read()
	go client.write()
	go client.timeouter()
	greeting := "Hello!\nI'll give you baaasic mathematic quiz. Let's begin!\n "
	quiz := make_quiz()
	client.question = "quiz:" + quiz
	client.outgoing <- greeting + quiz + " = ?\n"
	return client
}

func (client *Client) timeouter() {
	for {
		select {
		case <-client.ticker.C:
			if client.timeout {
				mes := "Close by Client's timeout\n"
				farewell := ">> timeout!\n"
				client.close(mes, farewell)
				return
			}
			client.timeout = true
		}
	}
}

func (client *Client) read() {
	for {
		line, err := client.reader.ReadString('\n')
		if err != nil {
			client.close(err.Error(), "")
			return
		}
		client.timeout = false
		client.incoming <- line
		fmt.Printf("[%s]Read:%s\n", client.conn.RemoteAddr(), line)
	}
}

func (client *Client) write() {
	for data := range client.outgoing {
		if data == "KILL CONNECTION" {
			fmt.Printf("[%s]KILL CONNECTION\n", client.conn.RemoteAddr())
			client.conn.Close()
			client = nil
			return
		}
		if _, err := client.writer.WriteString(data); err != nil {
			fmt.Printf("in writer WriteString: %s\n", err.Error())
			return
		}
		if err := client.writer.Flush(); err != nil {
			fmt.Printf("in writer flush: %s\n", err.Error())
			return
		}
		fmt.Printf("[%s]Write:%s\n", client.conn.RemoteAddr(), data)
	}
}

func (client *Client) close(mes string, farewell string) {
	if slt := str.Split(mes, " use "); len(slt) > 1 && slt[1] == "of closed network connection" {
		fmt.Println("in closed network conn")
	} else {
		fmt.Printf("[%s]%s\n", client.conn.RemoteAddr(), mes)
		client.outgoing <- farewell
		client.outgoing <- "KILL CONNECTION"
	}
}

func newListener() *net.TCPListener {
	service := ":8888"
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	checkError(err, "Resolve Error")
	listener, err := net.ListenTCP("tcp", tcpAddr)
	checkError(err, "Listen Error")
	fmt.Printf("Server Run Port: %s\n", service)
	return listener
}

func newTCPServer() *Server {
	listener := newListener()
	server := &Server{
		listener: listener,
		clients:  make([]*Client, 10),
		conn:     make(chan net.Conn),
		incoming: make(chan string),
		outgoing: make(chan string),
	}
	return server
}

func (server *Server) acceptLoop() {
	defer server.listener.Close()

	fmt.Println("Ready For Accept")
	for {
		conn, err := server.listener.Accept()
		checkError(err, "Accept Error")
		server.conn <- conn
	}
}

func (server *Server) listen() {
	fmt.Println("Ready For Listen")
	for {
		select {
		case conn := <-server.conn:
			server.addClient(conn)
		case data := <-server.incoming:
			server.response(data)
		}
	}
}

func (server *Server) addClient(conn net.Conn) {
	fmt.Printf("[%s]Accept\n", conn.RemoteAddr())
	client := newClient(conn, len(server.clients))
	server.clients = append(server.clients, client)
	go func() {
		for {
			select {
			case income := <-client.incoming:
				message := strconv.Itoa(client.idx) + ":" + income
				server.incoming <- message
			case outgo := <-server.outgoing:
				client.outgoing <- outgo
			}
		}
	}()
}

func (server *Server) response(data string) {
	idx, err := strconv.Atoi(str.Split(data, ":")[0])
	checkError(err, "in response: client index error!")
	bfr := str.Split(server.clients[idx].question, ":")
	if bfr[0] == "quiz" {
		ans := strconv.Itoa(calc_quiz(bfr[1]))
		next := make_quiz()
		if strconv.Itoa(idx)+":"+ans+"\n" == data {
			server.clients[idx].corrects += 1
			if server.clients[idx].corrects == pass_points {
				mes := "Close by Client's. Success!! 88888\n"
				farewell := "Congratulations!! this is flag:" + get_flag()
				server.clients[idx].close(mes, farewell)
			} else {
				data = ">>correct! ok, next question!\n " + next
				server.clients[idx].question = "quiz:" + next
				data += " = ?\n"
				server.clients[idx].outgoing <- data
			}
		} else {
			mes := "Close by Client's mistake.\n"
			farewell := ">>you made a mistake. bye!!\n"
			server.clients[idx].close(mes, farewell)
		}
	}
}

func get_flag() string {
	type Query struct {
		Genre  string `json:"genre"`
		Num    string `json:"num"`
		Caught string `json:"caught"`
		Flag   string `json:"flag"`
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	http_client := &http.Client{Transport: tr}
	url := "https://evening-anchorage-52082.herokuapp.com/admin/get_row?pass=stickyfingers&genre=app&num=1"
	resp, err := http_client.Get(url)
	if err != nil {
		fmt.Println("in get_flag http.Get error!:" + err.Error())
	}
	defer resp.Body.Close()
	var q []Query
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		fmt.Println("in get_flag Decode error!:" + err.Error())
	}
	return q[0].Flag
}

func make_quiz() string {
	rand.Seed(time.Now().UnixNano())
	left := rand.Intn(100)
	right := rand.Intn(100)
	oper := " + "
	switch operi := rand.Intn(4); operi {
	case 1:
		oper = " - "
	case 2:
		oper = " * "
	case 3:
		oper = " / "
		left *= 3
		right += 1
	}
	return strconv.Itoa(left) + oper + strconv.Itoa(right)
}

func calc_quiz(data string) int {
	tkn := str.Split(data, " ")
	left, err := strconv.Atoi(tkn[0])
	checkError(err, "in calc_quiz: Atoi error!")
	right, err2 := strconv.Atoi(tkn[2])
	checkError(err2, "in calc_quiz: Atoi error!")
	ans := left + right
	switch tkn[1] {
	case "-":
		ans = left - right
	case "*":
		ans = left * right
	case "/":
		ans = left / right
	}
	return ans
}

func checkError(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s", msg, err.Error())
		os.Exit(1)
	}
}

func main() {
	server := newTCPServer()
	go server.listen()
	server.acceptLoop()
}
