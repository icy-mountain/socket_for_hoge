package main

import (
	"bufio"
	"context"
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

//kWait: Server waits "kWait" second until client's timeout. default 5
//kPassPoints: Server pass client if client solves "kPassPoints" problems. default 100
const (
	kWait       = 5
	kPassPoints = 1
)

type Client struct {
	idx      int                /* manage client by "idx" */
	corrects int                /* count correct answers by "corrects" */
	timeout  bool               /* timeout or not */
	question string             /* problem text asking calculation */
	incoming chan string        /* client' meaasage -> client.incoming -> server.incoming -> to server */
	outgoing chan string        /* server's message -> server.outgoing -> client.outgoing -> to client */
	conn     net.Conn           /* connection killed by client's problem-mistake or timeout */
	ticker   *time.Ticker       /* to manage timeout */
	reader   *bufio.Reader      /* use in client.read() */
	writer   *bufio.Writer      /* use in client.write() */
	cancel   context.CancelFunc /* call cancel in cilent.close(), client.write() */
}

type Server struct {
	conn_ch  chan net.Conn
	incoming chan string
	outgoing chan string
	listener *net.TCPListener
	clients  []*Client
}

//NewClient: return new client to server
func newClient(conn net.Conn, length int, ctx context.Context, cancel context.CancelFunc) *Client {
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)
	ticker := time.NewTicker(kWait * time.Second)
	client := &Client{
		idx:      length,
		corrects: 0,
		timeout:  true,
		question: "",
		incoming: make(chan string),
		outgoing: make(chan string, 20),
		conn:     conn,
		ticker:   ticker,
		reader:   reader,
		writer:   writer,
		cancel:   cancel,
	}

	_ = getFlag() /* to wake up sleeping API sever */
	go client.read(ctx)
	go client.write(ctx)
	go client.timeOuter(ctx)
	greeting := "Hello!\nI'll give you baaasic mathematic quiz. Let's begin!\n "
	quiz := makeQuiz()
	client.question = "quiz:" + quiz
	client.outgoing <- greeting + quiz + " = ?\n"
	return client
}

//timeOuter close connection if client "kWait"sec over.
func (client *Client) timeOuter(ctx context.Context) {
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
		case <-ctx.Done():
			return
		}
	}
}

func (client *Client) read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
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
}

func (client *Client) write(ctx context.Context) {
	for {
		select {
		case data := <-client.outgoing:
			if _, err := client.writer.WriteString(data); err != nil {
				fmt.Printf("in writer WriteString: %s\n", err.Error())
				client.cancel()
				return
			}
			if err := client.writer.Flush(); err != nil {
				fmt.Printf("in writer flush: %s\n", err.Error())
				client.cancel()
				return
			}
			fmt.Printf("[%s]Write:%s\n", client.conn.RemoteAddr(), data)
		case <-ctx.Done():
			fmt.Printf("[%s]KILL CONNECTION\n", client.conn.RemoteAddr())
			client.conn.Close()
			client = nil
			return
		}
	}
}

func (client *Client) close(mes string, farewell string) {
	if spt := str.Split(mes, " use "); len(spt) > 1 && spt[1] == "of closed network connection" {
		fmt.Println("from read() in closed network conn err")
	} else {
		fmt.Printf("[%s]%s\n", client.conn.RemoteAddr(), mes)
		client.outgoing <- farewell
		client.cancel()
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
		conn_ch:  make(chan net.Conn),
		incoming: make(chan string),
		outgoing: make(chan string),
		listener: listener,
		clients:  make([]*Client, 10),
	}
	return server
}

func (server *Server) acceptLoop() {
	defer server.listener.Close()

	fmt.Println("Ready For Accept")
	for {
		conn, err := server.listener.Accept()
		checkError(err, "Accept Error")
		server.conn_ch <- conn
	}
}

func (server *Server) listen() {
	fmt.Println("Ready For Listen")
	for {
		select {
		case conn := <-server.conn_ch:
			server.addClient(conn)
		case data := <-server.incoming:
			server.response(data)
		}
	}
}

//add client to server.
//link server's incoming/outgoing and client's incoming/outgoing.
func (server *Server) addClient(conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	fmt.Printf("[%s]Accept\n", conn.RemoteAddr())
	client := newClient(conn, len(server.clients), ctx, cancel)
	server.clients = append(server.clients, client)
	go func() {
		for {
			select {
			case income := <-client.incoming:
				message := strconv.Itoa(client.idx) + ":" + income
				server.incoming <- message
			case outgo := <-server.outgoing:
				client.outgoing <- outgo
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (server *Server) response(data string) {
	idx, err := strconv.Atoi(str.Split(data, ":")[0])
	checkError(err, "in response: client index error!")
	the_client := server.clients[idx]
	foo := str.Split(the_client.question, ":")
	mode := foo[0]

	if mode == "quiz" {
		responseQuiz(the_client, data)
	}
}

func responseQuiz(the_client *Client, data string) {
	idx, err := strconv.Atoi(str.Split(data, ":")[0])
	checkError(err, "in response: client index error!")
	prob_statement := str.Split(the_client.question, ":")[1]
	ans := strconv.Itoa(calcQuiz(prob_statement))
	ans = strconv.Itoa(idx)+":"+ans+"\n" 

	if ans == data {
		the_client.corrects += 1
		if the_client.corrects == kPassPoints {
			mes := "Close by Client's. Success!! 88888\n"
			farewell := "Congratulations!! this is flag:" + getFlag()
			the_client.close(mes, farewell)
		} else {
			next := makeQuiz()
			mes := ">>correct! ok, next question!\n " + next + " = ?\n"
			the_client.question = "quiz:" + next
			the_client.outgoing <- mes
		}
	} else {
		mes := "Close by Client's mistake.\n"
		farewell := ">>you made a mistake. bye!!\n"
		the_client.close(mes, farewell)
	}
}

func getFlag() string {
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
	url := "https://evening-anchorage-52082.herokuapp.com/admin/get_row?pass=" + os.Getenv("CTF_PASS") + "&genre=app&num=1"
	resp, err := http_client.Get(url)
	if err != nil {
		fmt.Println("in getFlag http.Get error!:" + err.Error())
	}
	defer resp.Body.Close()
	var q []Query
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		fmt.Println("in getFlag Decode error!:" + err.Error())
	}
	return q[0].Flag
}

func makeQuiz() string {
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

// TODO: if Atoi error happen, return err. Don't panic().
func calcQuiz(data string) int {
	tkn := str.Split(data, " ")
	left, err := strconv.Atoi(tkn[0])
	checkError(err, "in calcQuiz: Atoi error!")
	right, err2 := strconv.Atoi(tkn[2])
	checkError(err2, "in calcQuiz: Atoi error!")
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
		panic(fmt.Sprintf("%s: %s", msg, err.Error()))
	}
}

func main() {
	if os.Getenv("CTF_PASS") == "" {
		fmt.Println("you need to set $CTF_PASS!!")
		return
	}
	server := newTCPServer()
	go server.listen()
	server.acceptLoop()
}
