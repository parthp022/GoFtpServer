package main

import (
	"GoFtpServer/common"
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

func putFile(conn *bufio.ReadWriter, file *os.File, source string, conntype int) {

	// Send the command
	err := common.WriteMessage("put", source, conn)
	if err != nil {
		fmt.Println("Error writing command:", err)
		return
	}

	// Read the status response
	command, value, err := common.ReadMessage(conn.Reader)
	if err != nil {
		fmt.Println("Error reading status:", err)
		return
	}

	if command == "ok" {

		fmt.Printf("Status OK, starting transfer of %d bytes\n", source)

		buffer := make([]byte, 1024)
		if err != nil {
			fmt.Println("Client file error:", file)
			return
		}
		if(conntype==0){
			for {
				n, err := file.Read(buffer)

				if err == io.EOF {
					fmt.Println("Transfer complete")
					break
				}

				bytes := []byte(buffer)
				_, err = conn.Write(bytes[0:n])

			}
		} else if(conntype ==1){
			for {
					file := context.file
					buffer := make([]byte, 1024)
					// Read into buffer
					len, err := file.Read(buffer)

					if len > 0 {
						// Write to socket
						bytes := []byte(buffer)
						_, err = conn.Write(bytes[0:n])
						if err != nil {
							log.Println("Failed to write data to socket; abandoning transfer")
						}
					}

					// EOF?
					if err == io.EOF {
						log.Println("Transfer completed")
						break
					}

					// Some other error
					if err != nil {
						log.Println("Error reading file; abandoning transfer")
						return
					}
					for{
						if message != "put-ack" {
							log.Println("Protocol error: expected get-ack")
							continue
						}	
						break
					}
			}
		}
		command, value, err := common.ReadMessage(conn.Reader)
		if err != nil {
			fmt.Println("Error reading status:", err)
			return
		}
		if command == "error" {
			fmt.Println("Server sent error:", value)
			return
		}
	} else if command == "error" {
		fmt.Println("Server sent error:", value)
		return
	} else {
		fmt.Printf("Unexpected message type '%s'\n", command)
		return
	}
}

func putFileTcp(source string, file *os.File, endpoint string) {

	fmt.Printf("Connecting to %s\n", endpoint)
	conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
	if err != nil {
		fmt.Println(err)
		return
	}

	readWriter := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriterSize(conn, 1))
	putFile(readWriter, file, source,0)
}

func putFileUdp(source string, file *os.File, endpoint string) {
	_, err := net.Dial("udp", endpoint)
	if err != nil {
		fmt.Printf("Fail to initiate connection; aborting\n")
		return
	}
	readWriter := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriterSize(conn, 1))
	putFile(readWriter, file, source,1)
}

func main() {
	var port int
	var host string
	var udp bool

	flag.BoolVar(&udp, "udp", false, "Enable UDP transfer")
	flag.IntVar(&port, "port", 8081, "Remote port to connect to")
	flag.StringVar(&host, "host", "localhost", "The remote server to connect to (ip or hostname)")
	flag.Parse()

	filePath := flag.Arg(0)

	if filePath == "" {
		fmt.Println("Please specify a file")
		return
	}

	fileName := filepath.Base(filePath)

	file, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("Failed to open file for writing: %s\n", err)
		return
	}
	stat, err := file.Stat()
	if err != nil {
		fmt.Printf("Failed to retrieve file info: %s\n", err)
		return
	}

	bytes := stat.Size()
	source := fmt.Sprintf("%s %d", fileName, bytes)
	endpoint := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Upload file '%s' on '%s' to '.\\%s'\n", filePath, endpoint, fileName)
	putFileTcp(source, file, endpoint)

	file.Close()
}
