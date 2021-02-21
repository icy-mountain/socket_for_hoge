package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
	str "strings"

	// "golang.org/x/net/context"
)

const (
	wait = 3
)

type Client struct {
	idx      int
	question string
	flag	 bool
	incoming chan string
	outgoing chan string
	end		 chan bool
	conn     net.Conn
	ticker	 *time.Ticker
	reader   *bufio.Reader
	writer   *bufio.Writer
}

type Server struct {
	conn     chan net.Conn
	incoming chan string
	outgoing chan string
	listener *net.TCPListener
	clients  []*Client
}

func newClient(connection net.Conn, length int) *Client {
	writer := bufio.NewWriter(connection)
	reader := bufio.NewReader(connection)
	ticker := time.NewTicker(wait * time.Second)

	client := &Client{
		conn:     connection,
		idx:      length,
		flag:	  false,
		incoming: make(chan string),
		outgoing: make(chan string),
		end:	  make(chan bool, 2),
		ticker:	  ticker,
		reader:   reader,
		writer:   writer,
	}

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
				fmt.Println("tick!")
				if !client.flag {
					fmt.Println("false!")
					client.outgoing <- "timeout!\n"
					client.ticker.Stop()
					client.close("Close by Client's timeout\n")
					return
				}
				client.flag = false
		}
	}
}

func (client *Client) read() {
	for {
		select {
			case <-client.end:
				client.close("Close by Client's mistake\n")
				return
			default:
				line, err := client.reader.ReadString('\n')
				if err != nil {
					client.close(err.Error())
					return
				}
				client.flag = true
				client.incoming <- line
				fmt.Printf("[%s]Read:%s\n", client.conn.RemoteAddr(), line)
		}
	}
}

func (client *Client) write() {
	for {
		data := <-client.outgoing
		if data == "KILL WRITER" {
			fmt.Printf("[%s]KILL WRITER\n", client.conn.RemoteAddr())
			return
		}
		if _, err  := client.writer.WriteString(data); err != nil {
			fmt.Printf("in writer WriteString: %s\n", err.Error())
		}
		if err  := client.writer.Flush(); err != nil {
			fmt.Printf("in writer flush: %s\n", err.Error())
		}
		fmt.Printf("[%s]Write:%s\n", client.conn.RemoteAddr(), data)
	}
}

func (client *Client) close(mes string) {
	if slt := str.Split(mes," use "); len(slt) > 1 && slt[1] ==  "of closed network connection" {
		fmt.Println("in closed network conn")
		return
	}	
	fmt.Printf("[%s]%s\n", client.conn.RemoteAddr(), mes)
	client.outgoing <- "KILL WRITER"
	client.conn.Close()
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

	go func() {
		for {
			select {
			case conn := <-server.conn:
				server.addClient(conn)
			case data := <-server.incoming:
				server.response(data)
			}
		}
	}()
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
			case outgo :=  <-server.outgoing:
				client.outgoing <- outgo
			}
		}
	}()
}

func (server *Server) response(data string) {
	idx , err := strconv.Atoi(str.Split(data, ":")[0])
	checkError(err, "in response: client index error!")
	bfr := str.Split(server.clients[idx].question, ":")
	if bfr[0] == "quiz" {
		ans := strconv.Itoa(calc_quiz(bfr[1]))
		next := make_quiz()
		if strconv.Itoa(idx) + ":" + ans  + "\n" == data {
			data = ">>correct! ok, next question!\n " + next
			server.clients[idx].question = "quiz:" + next
			data += " = ?\n"
			server.outgoing <- data
		} else {
			server.outgoing <- ">>you made a mistake. bye!!\n"
			server.clients[idx].end <- true
		}
	}
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
	left , err := strconv.Atoi(tkn[0])
	checkError(err, "in calc_quiz: Atoi error!")
	right , err2 := strconv.Atoi(tkn[2])
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
	server.listen()
	server.acceptLoop()
}
