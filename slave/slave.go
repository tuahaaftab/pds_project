package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
)

// Returns true if password found, else returns false
func searchPasswordInFile(password string, filename string) bool {

	file, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == password {
			return true
		}
	}

	return false
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
		fmt.Println(file.Name())
		fileNumberString := file.Name()[startNameSplit : len(file.Name())-endNameSplit]
		fileNumber, err := strconv.Atoi(fileNumberString)
		if err != nil {
			fmt.Println(err)
		}

		fileNumbers = append(fileNumbers, byte(fileNumber))
	}

	return fileNumbers
}

func main() {
	// Below code for searching password in files
	// passwordFound := searchPasswordInFile("!!!123blahblah!", "slave_files/passwords_1.txt")
	// fmt.Println("Password found: ", passwordFound)

	allFiles := make([][]byte, 3)
	// Reading numbers of files in slice
	allFiles[0] = getFileNumbersFromDirectory("slave_files")
	allFiles[1] = getFileNumbersFromDirectory("extra_files1")
	allFiles[2] = getFileNumbersFromDirectory("extra_files2")
	fmt.Println("All file numbers", allFiles)

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

	// writing fileNumbers at 0, 1, 2. 0 == slave_files, 1 == extra_files1, 2 == extra_files2
	for _, fileNumbersArray := range allFiles {
		//arrayToSend := make([]byte, numberOfFiles)
		temp := make([]byte, numberOfFiles)
		copy(temp, fileNumbersArray)
		arrayToSend = append(arrayToSend, temp...)
	}

	conn.Write(arrayToSend)
	fmt.Println("Sent array: ", arrayToSend)

	// conn should not close
	conn.Close()
}
