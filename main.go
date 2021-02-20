package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
	str "strings"
)

type Client struct {
	conn     net.Conn
	idx      int
	incoming chan string
	outgoing chan string
	question string
	Timeout  int
	reader   *bufio.Reader
	writer   *bufio.Writer
}

type Server struct {
	listener *net.TCPListener
	clients  []*Client
	conn     chan net.Conn
	incoming chan string
	outgoing chan string
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

func newClient(connection net.Conn, length int) *Client {
	writer := bufio.NewWriter(connection)
	reader := bufio.NewReader(connection)

	client := &Client{
		conn:     connection,
		idx:      length,
		incoming: make(chan string),
		outgoing: make(chan string),
		Timeout:  1,
		reader:   reader,
		writer:   writer,
	}

	go client.read()
	go client.write()
	greeting := "Hello!\nI\'ll give you baaasic mathematic quiz. Let\'s begin!\n "
	quiz := make_quiz()
	client.question = "quiz:" + quiz
	client.outgoing <- greeting + quiz + " = ?\n"
	return client
}

func (client *Client) read() {
	for {
		if client.Timeout > 0 {
			if err = conn.SetDeadline(time.Now().Add(client.Timeout)); err != nil {
				client.outgoing <- ">>you are too late! m9(^A^) BYE!\n"
				client.close("Close by client\' miss.\n")
				return
			}
		}
		line, err := client.reader.ReadString('\n')
		if err == io.EOF {
			client.close("catch EOF. Close.\n")
			return
		}
		if err != nil {
			client.close("read error. Close.\n")
			return
		}
		client.incoming <- line
		fmt.Printf("[%s]Read:%s", client.conn.RemoteAddr(), line)
	}
}

func (client *Client) write() {
	for data := range client.outgoing {
		if data == "KILL WRITER" {
			fmt.Printf("[%s]KILL WRITER\n", client.conn.RemoteAddr())
			return
		}
		client.writer.WriteString(data)
		client.writer.Flush()
		fmt.Printf("[%s]Write:%s\n", client.conn.RemoteAddr(), data)
	}
}

func (client *Client) close(mes: string) {
	fmt.Printf("[%s]%s", client.conn.RemoteAddr(), mes)
	client.conn.Close()
	client.outgoing <- "KILL WRITER"
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
		clients:  make([]*Client, 0),
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
			message := strconv.Itoa(client.idx) + ":" + <-client.incoming
			server.incoming <- message
			client.outgoing <- <-server.outgoing
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
		if strconv.Itoa(idx) + ":" + ans  + "\n" != data {
			server.clients[idx].outgoing <- ">>you made a mistake! m9(^A^) BYE\n"
			server.clients[idx].close("Close by client\'s mistake\n");
			return
		}
		data = ">>correct! ok, next question!\n " + next
		server.clients[idx].question = "quiz:" + next
	}
	data += " = ?\n"
	server.outgoing <- data
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
