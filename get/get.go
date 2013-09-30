package main

import (
	"GoFtpServer/common"
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type GetFileRequest struct {
	File       *os.File
	Socket     *bufio.ReadWriter
	FileName   string
	Endpoint   string
	VerifyMode bool
	Udp        bool
}

func GetFileCore(request *GetFileRequest) {

	conn := request.Socket
	file := request.File

	commandParameters := request.FileName
	if request.VerifyMode {
		commandParameters = fmt.Sprintf("%s -v", commandParameters)
	}

	// Send the command
	err := common.WriteMessage("get", commandParameters, conn)
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

	if command == "error" {
		fmt.Println("Server sent error:", value)
		return
	}

	if command != "length" {
		fmt.Printf("Unexpected message type '%s'\n", command)
		return
	}

	segments := strings.Split(value, " ")

	var hash string
	if request.VerifyMode {
		hash = segments[1]
	}

	length, err := strconv.Atoi(segments[0])
	if err != nil {
		fmt.Printf("Protocol error: %s. DEBUG: %s", err, segments[0])
		return
	}

	fmt.Printf("Status OK, starting transfer of %d bytes\n", length)

	hasher := md5.New()

	totalRead := 0
	buffer := make([]byte, 1024)
	for totalRead < length {
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

		if request.Udp {
			conn.Write([]byte("get-ack"))
		}

		hasher.Write(buffer[:read])

		totalRead += read
	}

	if totalRead == length {
		fmt.Println("Transfer complete")
		realHash := hex.EncodeToString(hasher.Sum([]byte{}))
		if request.VerifyMode && (len(realHash) == 0 || len(hash) == 0) {
			fmt.Println("Checksum calculation failed")
		} else if request.VerifyMode {
			if realHash == hash {
				fmt.Printf("Checksum OK")
			} else {
				fmt.Println("Checksum failed; file is corrupt")
				fmt.Println("Expected: ", hash)
				fmt.Println("Actual: ", realHash)
			}
		}
	} else {
		fmt.Println("Connection closed before transfer completed. File contents are likely incomplete.")
	}
}

func GetFile(request *GetFileRequest) {

	var connectionType string
	if request.Udp {
		connectionType = "udp"
	} else {
		connectionType = "tcp"
	}

	endpoint := request.Endpoint
	fmt.Printf("Connecting to %s (%s)\n", endpoint, connectionType)
	conn, err := net.DialTimeout(connectionType, endpoint, 5*time.Second)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Questionable decision -- use buffered I/O for every read. This is so we can use bufio.Reader.ReadString to read
	// newline terminated strings easily.
	// P.S. using NewWriterString with size 1 to force flush to the socket every writer.
	// P.P.S. the read buffer must be 1024 to make sure we can read UDP datagrams
	request.Socket = bufio.NewReadWriter(bufio.NewReaderSize(conn, 1024), bufio.NewWriterSize(conn, 1))
	GetFileCore(request)
	conn.Close()
}

func main() {
	var port int
	var host string
	var udp, verify bool

	flag.BoolVar(&udp, "udp", false, "Enable UDP transfer")
	flag.BoolVar(&verify, "v", false, "Enable verify mode (checksum)")
	flag.IntVar(&port, "port", 8081, "Remote port to connect to")
	flag.StringVar(&host, "host", "localhost", "The remote server to connect to (ip or hostname)")
	flag.Parse()

	filePath := flag.Arg(0)

	if filePath == "" {
		fmt.Println("Please specify a file")
		return
	}

	fileName := filepath.Base(filePath)

	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0)
	if err != nil {
		fmt.Printf("Failed to open file for writing: %s\n", err)
		return
	}

	endpoint := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Download file '%s' on '%s' to '.\\%s'\n", filePath, endpoint, fileName)

	request := &GetFileRequest{file, nil, fileName, endpoint, verify, udp}
	GetFile(request)
	file.Close()
}
