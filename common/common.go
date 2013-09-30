package common

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

//
//  Write a string to a socket. Write the raw bytes first, then terminate with a newline.
//
func WriteString(conn io.Writer, str string) error {
	value := fmt.Sprintf("%s\n", str)
	bytes := []byte(value)
	_, err := conn.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}

func WriteMessage(command string, value string, conn io.Writer) error {
	var message string
	if value == "" {
		message = command
	} else {
		message = fmt.Sprintf("%s: %s", command, value)
	}

	err := WriteString(conn, message)
	return err
}

func ParseMessage(value string) (string, string) {
	value = strings.Trim(value, "\n")

	var command, parameter string

	index := strings.Index(value, ":")
	if index < 0 {
		command = value
		parameter = ""
	} else {
		command = value[:index]
		parameter = value[index+2:]
	}

	return command, parameter
}

func ReadMessage(reader *bufio.Reader) (string, string, error) {
	value, err := ReadString(reader)
	if err != nil {
		return "", "", err
	}

	command, parameter := ParseMessage(value)
	return command, parameter, nil
}

func ReadString(reader *bufio.Reader) (string, error) {
	str, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	str = str[:len(str)-1]
	return str, nil
}

func WriteError(conn io.Writer, err error) {
	errorMessage := fmt.Sprintf("error: %s", err)
	WriteString(conn, errorMessage)
}

func GetHash(file *os.File) (string, error) {
	current, err := file.Seek(0, 1)
	if err != nil {
		return "", err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", err
	}

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	buffer := make([]byte, stat.Size())
	_, err = io.ReadFull(file, buffer)
	if err != nil {
		return "", err
	}

	_, err = file.Seek(current, 0)
	if err != nil {
		return "", err
	}

	hasher := md5.New()
	hasher.Write(buffer)
	hash := hasher.Sum([]byte{})

	hashString := hex.EncodeToString(hash)
	return hashString, nil
}
