package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"time"
)

// Returns true if password found, else returns false
func searchPasswordInFile(password string, fileNumber byte, priority byte) byte {
	fileNumberString := strconv.Itoa(int(fileNumber))
	filename := ""
	if priority == 1 {
		filename = "slave_files/passwords_" + string(fileNumberString) + ".txt"
	} else if priority == 2 {
		filename = "extra_files1/passwords_" + string(fileNumberString) + ".txt"
	} else if priority == 3 {
		filename = "extra_files2/passwords_" + string(fileNumberString) + ".txt"
	}

	fmt.Println("Searching "+password+" in ", filename)

	file, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// TODO
		//passwords_8.txt not being scanned completely
		if fileNumber == 8 {
			//fmt.Println(scanner.Text())
		}

		if scanner.Text() == password {
			fmt.Println(password+" found in file ", filename)
			return 1
		}
	}

	return 0
}

// Returns file numbers of password files present in given directory
func getFileNumbersFromDirectory(directoryName string) []byte {
	files, err := ioutil.ReadDir("./" + directoryName)
	if err != nil {
		fmt.Println(err)
	}

	fileNumbers := make([]byte, 0)
	startNameSplit := len("passwords_")
	endNameSplit := len(".txt")
	for _, file := range files {
		fileNumberString := file.Name()[startNameSplit : len(file.Name())-endNameSplit]
		fileNumber, err := strconv.Atoi(fileNumberString)
		if err != nil {
			fmt.Println(err)
		}

		fileNumbers = append(fileNumbers, byte(fileNumber))
	}

	return fileNumbers
}

func receiveSearchRequests(conn net.Conn) {
	// 1st byte of buffer tells type of message
	//searchFileCommand, quitSearchCommand, heartbeatCommand := 1, 2, 3

	buf := make([]byte, 100)

	for {
		// getting search request from server
		_, err := conn.Read(buf)
		fmt.Println("Receiving search request")

		if err != nil {
			fmt.Println(err)
			// continue
		}

		searchFileCommand := buf[0]
		priority := buf[1]
		fileNumber := buf[2]
		lenPassword := buf[3]
		passwordToSearch := string(buf[4 : lenPassword+4])

		searchResult := searchPasswordInFile(passwordToSearch, fileNumber, priority)

		if searchResult == 0 {
			fmt.Println("Password NOT found in file: ", fileNumber)
		} else if searchResult == 1 {
			fmt.Println("Password ***FOUND*** in file: ", fileNumber)
		}

		buf[0] = searchFileCommand
		// searchResult == 0(notFound), 1(found), 2(error)
		buf[1] = searchResult

		// sending searchResult to server
		_, err = conn.Write(buf)
		if err != nil {
			fmt.Println(err)
			// continue
		}

	}

}

func main() {
	// allFiles present with the slave node
	allFiles := make([][]byte, 3)
	// Reading numbers of files in slice
	allFiles[0] = getFileNumbersFromDirectory("slave_files")
	allFiles[1] = getFileNumbersFromDirectory("extra_files1")
	allFiles[2] = getFileNumbersFromDirectory("extra_files2")

	conn, err := net.Dial("tcp", "localhost:5555")
	if err != nil {
		fmt.Println(err)
	}

	// numberOfFiles represents the max files in a folder
	//change this according to files in a folder
	// TODO: change if max number of files change in a folder
	numberOfFiles := 10
	// First byte to server will tell it length of each fileNumbersArray. fileNumbersArray declared in below for loop
	arrayToSend := []byte{byte(numberOfFiles)}

	// writing fileNumbers at 0, 1, 2.    0 == slave_files, 1 == extra_files1, 2 == extra_files2
	for _, fileNumbersArray := range allFiles {
		//arrayToSend := make([]byte, numberOfFiles)
		temp := make([]byte, numberOfFiles)
		copy(temp, fileNumbersArray)
		arrayToSend = append(arrayToSend, temp...)
	}

	// informing server about files present with array
	fmt.Println("Sent array: ", arrayToSend)
	conn.Write(arrayToSend)

	// receives search requests from server
	go receiveSearchRequests(conn)

	// conn should not close
	//conn.Close()

	//prevent slave from exiting
	time.Sleep(500 * time.Second)
}
