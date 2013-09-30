package main

import (
	"GoFtpServer/common"
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

func ReadFileHandler(parameters string, connection io.ReadWriter) {

	args := strings.Split(parameters, " ")

	var verify bool
	flagSet := flag.NewFlagSet("", flag.ContinueOnError)
	flagSet.BoolVar(&verify, "v", false, "Verify")
	flagSet.Parse(args[1:])

	fileName := args[0]

	log.Printf("Request for file '%s'", fileName)

	file, err := os.Open(fileName)
	if err != nil {
		common.WriteError(connection, err)
		return
	}

	stat, err := file.Stat()
	if err != nil {
		common.WriteError(connection, err)
		file.Close()
		return
	}

	hash, err := common.GetHash(file)
	if err != nil {
		log.Println("Hash calculation failed")
		file.Close()
		return
	}

	lengthParameters := fmt.Sprintf("%d", stat.Size())
	if verify {
		lengthParameters = fmt.Sprintf("%s %s", lengthParameters, hash)
	}

	common.WriteMessage("length", lengthParameters, connection)

	buffer := make([]byte, 1024)
	for {
		// Read into buffer
		len, err := file.Read(buffer)

		// EOF?
		if err == io.EOF {
			log.Println("Transfer completed")
			break
		}

		// Some other error
		if err != nil {
			log.Println("Error reading file; abandoning transfer")
			break
		}

		// Write to socket
		_, err = connection.Write(buffer[:len])
		if err != nil {
			log.Println("Failed to write data to socket; abandoning transfer")
		}
	}

	file.Close()
}

func WriteFileHandler(parameters string, connection io.ReadWriter) {

	segments := strings.Split(parameters, " ")

	if len(segments) != 2 {
		log.Println("Invalid put command formatting")
	}

	lengtha, _ := strconv.Atoi(segments[1])
	fileName := segments[0]

	log.Printf("Receive file '%s' (length = %d)", fileName, lengtha)

	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		log.Printf("Failed to open file for writing: %s\n", err)
		return
	}

	common.WriteString(connection, "ok")

	buffer := make([]byte, 1024)

	for {

		// Read from socket
		n, err := connection.Read(buffer)
		if err != nil {
			log.Println("Failed to read data from socket; abandoning transfer")
			break
		}

		// Read into buffer
		_, err = file.Write(buffer[:n])

		lengtha = lengtha - n
		if lengtha == 0 {
			log.Println("Transfer completed")
			break
		}
	}

	common.WriteString(connection, "ok")
	file.Close()
}

func HandleRequest(conn net.Conn) {

	readWriter := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriterSize(conn, 1))

	command, parameters, err := common.ReadMessage(readWriter.Reader)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}

	if command == "get" {
		ReadFileHandler(parameters, readWriter)
	} else if command == "put" {
		WriteFileHandler(parameters, readWriter)
	} else {
		log.Printf("Unknown command '%d'", command)
	}

	conn.Close()
}

func ListenConnections(listenSocket net.Listener) {
	for {
		connection, error := listenSocket.Accept()
		log.Println("Accepted connection")
		if error != nil {
			log.Fatal(error)
			continue
		}

		go HandleRequest(connection)
	}
}

type UdpContext struct {
	dest       net.Addr
	length     int
	command    string
	parameters string
	file       *os.File
}

func GetSend(conn *net.UDPConn, context *UdpContext) {
	file := context.file
	buffer := make([]byte, 1024)
	// Read into buffer
	len, err := file.Read(buffer)

	if len > 0 {
		// Write to socket
		_, err = conn.WriteTo(buffer[:len], context.dest)
		if err != nil {
			log.Println("Failed to write data to socket; abandoning transfer")
		}
	}

	// EOF?
	if err == io.EOF {
		log.Println("Transfer completed")
		return
	}

	// Some other error
	if err != nil {
		log.Println("Error reading file; abandoning transfer")
		return
	}
}
func PutRecv(conn *net.UDPConn, context *UdpContext) (int){
	file := context.file
	buffer := make([]byte, 1024)
	read, err := conn.Read(buffer)

	if err == io.EOF {
		break
	}

	if err != nil {
		fmt.Printf("Error reading from network: %s. Aborting transfer\n", err)
		return
	}

	_, err = file.Write(buffer[:read])
	if err != nil {
		fmt.Println("Error writing to file; aborting transfer")
		return
	}

	_, err = conn.Write([]byte("put-ack"))
	if err != nil {
		fmt.Println("Error writing ack; aborting transfer")
		return
	}

	return read;	
}


func HandleUdpGet(conn *net.UDPConn, context *UdpContext) {

	log.Printf("Request for file '%s' (UDP)", context.parameters)

	command := fmt.Sprintf("length: %d\n", context.length)
	conn.WriteTo([]byte(command), context.dest)

	GetSend(conn, context)
}

func HandleUdpput(conn *net.UDPConn, context *UdpContext) {

	log.Printf("Recieving file '%s' (UDP)", context.parameters)

	command := fmt.Sprintf("length: %d\n", context.length)

	context.length = context.length - PutRecv(conn, context)
}

func CreateContext(addr net.Addr, message string) (*UdpContext, error) {
	command, parameters := common.ParseMessage(message)

	if command == "get" {
		file, err := os.Open(parameters)
		if err != nil {
			return nil, err
		}

		stat, err := file.Stat()
		if err != nil {
			return nil, err
		}

		length := stat.Size()

		context := &UdpContext{addr, int(length), command, parameters, file}
		return context, nil
	} else if command == "put" {
		segments := strings.Split(parameters, " ")
		file, err := os.OpenFile(segments[0], os.O_RDWR|os.O_CREATE, 0)
		if err != nil {
			fmt.Printf("Failed to open file for writing: %s\n", err)
			return
		}
		length, _ := strconv.Atoi(segments[1])
		context := &UdpContext{addr, int(length), command, parameters, file}
	}

	return nil, nil
}

func ReadUdpConnections(conn *net.UDPConn) {

	contexts := make(map[string]*UdpContext)

	var buffer [1024]byte
	for {
		n, addr, err := conn.ReadFrom(buffer[:])
		if err != nil {
			log.Printf("Failed to read from udp socket: %s\n", err)
			continue
		}

		key := addr.String()

		context := contexts[key]

		if context == nil {
			// New request
			message := string(buffer[:n])
			context, err := CreateContext(addr, message)
			if err != nil {
				log.Printf("Failed to start udp session: %s\n", err)
				continue
			}

			contexts[key] = context

			// Start
			if context.command == "get" {
				HandleUdpGet(conn, context)
			}
			if context.command == "get" {
				HandleUdpPut(conn, context)
			}
		} else if context.command == "get" {
			message := string(buffer[:n])

			// Flow-control: only allow one outstanding message at a time. Wait for an ack after every request.
			if message != "get-ack" {
				log.Println("Protocol error: expected get-ack")
				continue
			}
			GetSend(conn, context)
		} else if context.command == "put" {
			
			message := string(buffer[:n])
			context.length = context.length - PutRecv(conn, context)
		}
	}
}

func StartTcp(endpoint string) {
	listenSocket, error := net.Listen("tcp", endpoint)
	if error != nil {
		log.Fatal(error)
	}

	log.Printf("Accepting TCP connections (%s)", endpoint)
	go ListenConnections(listenSocket)
}

func StartUdp(endpoint string) {
	addr, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Accepting UDP connections (%s)", endpoint)

	go ReadUdpConnections(conn)
}

func main() {
	log.Println("Starting server")

	var port int
	flag.IntVar(&port, "port", 8081, "Remote port to connect to")
	flag.Parse()

	binding := fmt.Sprintf(":%d", port)

	StartTcp(binding)
	StartUdp(binding)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()

		command := strings.ToLower(scanner.Text())
		if command == "exit" || command == "quit" {
			fmt.Println("Bye!")
			break
		}
	}
}

//
//  UDP stuff below
//  Basic idea is a way to abstract a connection
//
// type udpSession struct {
// 	key string
// 	*udpSessionManager
// 	net.Addr
// }

// func (session *udpSession) Read(bytes []byte, timeout int) (int, error) {
// 	manager := session.udpSessionManager
// 	cond := manager.Cond

// 	cond.L.Lock()

// 	for {
// 		// We have the lock at this point
// 		data := manager.tryRead(session.key)
// 		if data != nil {
// 			// Unlock
// 			cond.L.Unlock()
// 			// Copy and reutnr
// 			n := len(data)
// 			copy(bytes, data)
// 			return n, nil
// 		}

// 		// Release lock & wait
// 		cond.Wait()
// 		// Lock is acquired at this point
// 	}
// }

// type udpSessionManager struct {
// 	*net.UDPConn
// 	*sync.Cond

// 	sessions        map[string]*udpSession
// 	pendingMessages map[string][][]byte
// }

// func NewManager(conn *net.UDPConn) *udpSessionManager {
// 	lock := new(sync.Mutex)
// 	cond := sync.NewCond(lock)
// 	manager := &udpSessionManager{conn, cond, make(map[string]*udpSession), make(map[string][][]byte)}

// 	return manager
// }

// func (manager *udpSessionManager) tryRead(key string) []byte {
// 	if manager.pendingMessages[key] == nil {
// 		return nil
// 	}

// 	pendingReads := manager.pendingMessages[key]
// 	// Get last entry
// 	data := pendingReads[0]
// 	// Remove from list
// 	manager.pendingMessages[key] = pendingReads[1:]

// 	return data
// }

// func (manager *udpSessionManager) getSession(addr net.Addr) *udpSession {

// 	var session *udpSession
// 	key := addr.String()
// 	if manager.sessions[key] == nil {
// 		session = &udpSession{addr.String(), manager, addr}
// 		manager.sessions[key] = session
// 	} else {
// 		session = manager.sessions[key]
// 	}

// 	return session
// }

// func (manager *udpSessionManager) process() {

// 	for {
// 		file, err := manager.UDPConn.File()
// 		fd := file.Fd()
// 		syscall.Recvfrom(fd, 0, syscall.MSG_PEEK)

// 	}
// }
