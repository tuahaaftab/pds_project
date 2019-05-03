package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Page is an html page
type Page struct {
	Title string
	Body  []byte
}

// loadPage loads contents of a file whose name is provided as argument
func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

// Slave contains informaion about a slave node
type Slave struct {
	// to take control of slave, mux.lock
	mux         sync.Mutex
	isAvailable bool // is slave available for work
	conn        net.Conn
	allFiles    [][]byte //0 index = numbers of slaves own files. 1 & 2 index have numbers of extra files.
}

// registerSlaves listens for new slaves on port 5555 and insert new conn in slavesChannel
func registerSlaves(slavesChannel chan net.Conn) {
	// listen port for slaves
	in, err := net.Listen("tcp", ":5555")
	if err != nil {
		fmt.Println(err)
	}

	for {
		conn, err := in.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		slavesChannel <- conn
	}
}

// makeNewSlave populates a slave using its conn and returns it
func makeNewSlave(conn net.Conn) (*Slave, error) {
	var slave Slave

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)

	if err != nil {
		fmt.Println(err)
		return nil, nil
	}

	// first byte received tell number of max files in a slaves file folder
	numberOfFiles := buf[0]

	// receiving arrays of file numbers
	// 2d slice at position 0 contaings slaves original files, other 2 positions are for extra files
	allFiles := make([][]byte, 3) // 3 rows in slice will be added
	allFiles[0] = make([]byte, numberOfFiles)
	allFiles[1] = make([]byte, numberOfFiles)
	allFiles[2] = make([]byte, numberOfFiles)
	buf = buf[1:n]
	copy(allFiles[0], buf[0:numberOfFiles])
	copy(allFiles[1], buf[numberOfFiles:2*numberOfFiles])
	copy(allFiles[2], buf[2*numberOfFiles:3*numberOfFiles])

	// adding to slave variables
	slave.isAvailable = true
	slave.conn = conn
	slave.allFiles = allFiles

	return &slave, nil
}

// addSlave adds a slave in slave array
func addSlave(slaves []*Slave, slave *Slave) {
	for index := range slaves {
		if slaves[index] == nil {
			slaves[index] = slave
			break
		}

	}
}

// slaveManager communicates with new slave about files info then adds slave to slaves
func slaveManager(slaves []*Slave, newSlavesChannel chan net.Conn) {

	for {
		select {
		case conn := <-newSlavesChannel:
			// making a new slave and populating using its conn
			slavePointer, err := makeNewSlave(conn)
			if err != nil {
				fmt.Println(err)
				continue
			}

			//append slave to all slaves
			//slave := *slavePointer
			addSlave(slaves, slavePointer)
			//slaves = append(slaves, slave)
			fmt.Println("All Slaves", slaves)

		default:
			// can have another case for when a slave leaves
			// can hangle heartbeat here
		}
	}
}

// Client contaings information about client
type Client struct {
	w                http.ResponseWriter
	r                *http.Request
	passwordToSearch string
}

// used in a handler to pass channel to handler
type newClientHandler struct {
	newClientsChannel chan Client
}

// newClientHandler is called by clientListener
// this functions makes new client and adds it to newClientsChannel
// the newClientMade also has password to search with it
func (nch *newClientHandler) ServeHTTP(writer http.ResponseWriter, reader *http.Request) {
	err := reader.ParseForm()
	if err != nil {
		fmt.Println(err)
		return
	}

	// body is name of text area in which password entered in form
	passToSearch := reader.Form.Get("body")

	// populating newClient
	newClient := Client{w: writer, r: reader, passwordToSearch: passToSearch}
	// adding to newClientsChannel. This client will be received in clientManager
	nch.newClientsChannel <- newClient

	// a response written to the client
	fmt.Fprintf(writer, "Hi there, I am searching for your password "+passToSearch+" %s!", reader.URL.Path[1:])
}

// Sends a form to web client to enter password into.
func mainPageHandler(w http.ResponseWriter, r *http.Request) {
	// TODO : Below code works. Need to check if it is supposed to be like this.
	t, _ := template.ParseFiles("MainPage.html")
	p, _ := loadPage("MainPage.html")
	// below send an empty form to client
	t.Execute(w, p)
}

// listens for client on port 8080
// ncc == newClientsChannel
func clientListener(ncc chan Client) {
	// handler function called when
	newClientHandler := &newClientHandler{newClientsChannel: ncc}

	// on first request call below handler which sends a form to client
	http.HandleFunc("/", mainPageHandler)
	// newClientHandler called when text area filled with password and form is submitter
	http.Handle("/searchPassword", newClientHandler)

	http.ListenAndServe(":8080", nil)
}

// clientManager receives newClient that are added into newClientsChannel by newClientHandler
func clientManager(clients []Client, newClientsChannel chan Client, slaves []*Slave, chanForStatusShower chan []byte) {
	for {
		select {
		case newClient := <-newClientsChannel:
			clients = append(clients, newClient)

			go searchPasswordForClient(newClient, slaves, chanForStatusShower)
			fmt.Println("All clients : ", clients)
		}
	}
}

// checks if all files present have been searched for password
func allSearched(searchedStatusOfFiles []byte) bool {
	//notSearching := byte(0)
	//searching := byte(1)
	searched := byte(2)

	for _, status := range searchedStatusOfFiles {
		if status != searched {
			return false
		}
	}

	return true
}

// return filesToSearch and needToSearch
func getFilesToSearch(searchedStatusOfFiles []byte, slave *Slave, priority int) ([]byte, bool) {
	// filenumber of files in a slaves folder. Three folders at each slave node
	fileNumbers := slave.allFiles[priority-1]

	notSearching := byte(0)
	searching := byte(1)
	//searched := byte(2)
	filesToSearch := make([]byte, 0)
	needToSearch := false

	// cheking for fileNumbers that have not been searched
	for _, fileNumber := range fileNumbers {
		// File number should start with 1
		if fileNumber == 0 {
			continue
		}

		if searchedStatusOfFiles[fileNumber] == notSearching {
			filesToSearch = append(filesToSearch, fileNumber)
			searchedStatusOfFiles[fileNumber] = searching //will tell slave to search when this function returns
			needToSearch = true
		}
	}

	return filesToSearch, needToSearch

}

func searchFilesInSlave(slave *Slave, filesToSearch []byte, searchedStatusOfFiles []byte, passwordToSearch string, priority int) {
	// Ignore : returns passwordFound, errorWhileSearching
	//notSearching, searching, searched := byte(0), byte(1), byte(2)
	notSearching, searched := byte(0), byte(2)
	// locking slave so only this functin can communicate with slave
	slave.mux.Lock()
	slave.isAvailable = false
	defer slave.mux.Unlock()

	// 1st byte of buffer tells type of message
	//searchFileCommand, quitSearchCommand, heartbeatCommand := 1, 2, 3
	searchFileCommand := byte(1)

	buf := make([]byte, 100)
	lenPassword := byte(len(passwordToSearch))
	conn := slave.conn

	// for all files to search
	for _, fileNumber := range filesToSearch {
		// passwrod and fileToSearc information to be sent to slave
		buf[0] = searchFileCommand
		buf[1] = byte(priority)
		buf[2] = fileNumber
		buf[3] = lenPassword
		copy(buf[4:lenPassword+4], []byte(passwordToSearch))

		// writing information for searchin to slave
		_, err := conn.Write(buf)
		if err != nil {
			fmt.Println(err)
			searchedStatusOfFiles[fileNumber] = notSearching
			continue
		}

		// reading search response from slave
		_, err = conn.Read(buf)
		if err != nil {
			fmt.Println(err)
			searchedStatusOfFiles[fileNumber] = notSearching
			continue
		}

		// extracting search response from slave
		if buf[0] == searchFileCommand {
			searchResult := buf[1] // searchResult == 0(notFound), 1(found), 2(error)
			notFound, found := byte(0), byte(1)

			if searchResult == notFound {
				searchedStatusOfFiles[fileNumber] = searched
			} else if searchResult == found {
				searchedStatusOfFiles[fileNumber] = searched

				// TODO : other action for stopping all search
			}
		}

	}

	// slave is availabel again
	slave.isAvailable = true
}

// searches password for client
func searchPasswordForClient(client Client, slaves []*Slave, chanForStatusShower chan []byte) {
	passwordToSearch := client.passwordToSearch

	// TODO: change IF total number of files change
	// +1 so easy comparison of index with file numbers
	totalNumberOfFiles := 57 + 1

	// files searced contains true for files searched. For file_1 index == 1 in slice
	// 0 == not searching, 1 == searching, 2 == searched
	notSearching := byte(0)
	searching := byte(1)
	//searched := byte(2)
	searchedStatusOfFiles := make([]byte, totalNumberOfFiles) //initally all are 0

	// statusShower periodically prints this to console
	chanForStatusShower <- searchedStatusOfFiles

	// while all files not searched for password
	for !allSearched(searchedStatusOfFiles) {
		// if priority == 1, all slaves priority 1 files are searched
		for priority := 1; priority <= 3; priority++ {
			// loop over all slaves
			for _, slavePointer := range slaves {
				// slavePointer == nil means slave not assigned in array at that index
				if slavePointer == nil {
					continue
				}

				// Ignore: slave := *slavePointer
				// if slave.isAvailable then mutex is not locked for slave
				if (*slavePointer).isAvailable {
					// needToSearch is true if slave has files in which password has not been searched.
					//  FilesToSearch are file numbers to search in slave
					filesToSearch, needToSearch := getFilesToSearch(searchedStatusOfFiles, slavePointer, priority)

					if needToSearch == true {
						//Todo :
						// searchFilesInSlave automatically updates searchedStatusOfFiles. (Conflicts?)
						go searchFilesInSlave(slavePointer, filesToSearch, searchedStatusOfFiles, passwordToSearch, priority)
						//passwordFound, errorWhileSearching := searchFilesInSlave(slave, filesToSearch, searchedFiles, priority)
					}

				}
			}
		}

		// may sleep a little before resetting. Sleep()
		// resetting status of files that are being searched to not_searching
		// gap before scanning all slaves again.
		time.Sleep(2 * time.Second)
		for index, status := range searchedStatusOfFiles {
			if status == searching {
				searchedStatusOfFiles[index] = notSearching
			}
		}
	}

}

// statusShower periodically shows status of searched files for a clients
func statusShower(chanForStatusShower chan []byte) {
	allStatuses := make([][]byte, 0)

	for {
		select {
		case searchedStatusOfFiles := <-chanForStatusShower:
			allStatuses = append(allStatuses, searchedStatusOfFiles)

		default:
			// loop and print all statuses
			for index, searchedStatusOfFiles := range allStatuses {
				// not_searching = 0, searching = 1, searched = 2
				fmt.Println("Client_"+strconv.Itoa(index+1)+" status: ", searchedStatusOfFiles)
			}

			time.Sleep(2 * time.Second)
		}
	}
}

func main() {

	// contain all registered slaves
	slaves := make([]*Slave, 10)
	// contain all current clients
	clients := make([]Client, 0)

	// new conn pushed in slavesChannel in registerSlaves and received in slaveManager
	newSlavesChannel := make(chan net.Conn)
	//new client pushed in newClientsChannel in newClientHandler and received in clientManager
	newClientsChannel := make(chan Client)

	// registerSlaves listens on port 5555 and incoming connection to newSlavesChannel
	go registerSlaves(newSlavesChannel)
	// slaveManager receives a new conn from newSlavesChannel and uses it to populate a new slave struct. Then adds it to slaves
	go slaveManager(slaves, newSlavesChannel)

	// shows state of search for all clients
	chanForStatusShower := make(chan []byte)

	// clientListener runs http server. Handles incoming requests concurrently.
	// new clients formed are inserted into newClientsChannel
	go clientListener(newClientsChannel)
	// clientManager receives new client from newClientsChannel and appends to clients
	go clientManager(clients, newClientsChannel, slaves, chanForStatusShower)

	go statusShower(chanForStatusShower)

	// stopping main from quitting
	time.Sleep(500 * time.Second)

}

/* Look into:
1) How to update slaves array so no conflict occurs. Add, delete slave

2) How to update searchedStatusOfFiles so no conflict occurs

3) Concurrency in slave

*) Solved
Instead of creating a slice of 0 len and appending slaves, create a slice of some length
and add new slaves on empty positions. Then if a slice passed to function, that function will know
the lengt of the slice and can loop over it.
Solution : created a slice of particular length and passed. Every one can loop over slice of same length





*/
