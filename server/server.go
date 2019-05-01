package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
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

// slaveManager communicates with new slave about files info then adds slave to slaves
func slaveManager(slaves []Slave, newSlavesChannel chan net.Conn) {

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
			slave := *slavePointer
			slaves = append(slaves, slave)
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
	fmt.Fprintf(writer, "Yo yo yo "+passToSearch+" %s!", reader.URL.Path[1:])
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
func clientManager(clients []Client, newClientsChannel chan Client) {
	for {
		select {
		case newClient := <-newClientsChannel:
			clients = append(clients, newClient)
			fmt.Println("All clients : ", clients)
		}
	}
}

func main() {

	// contain all registered slaves
	slaves := make([]Slave, 0)
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

	// clientListener runs http server. Handles incoming requests concurrently.
	// new clients formed are inserted into newClientsChannel
	go clientListener(newClientsChannel)
	// clientManager receives new client from newClientsChannel and appends to clients
	go clientManager(clients, newClientsChannel)

	// stopping main from quitting
	for {

	}

}

/* Look into:
1) Instead of creating a slice of 0 len and appending slaves, create a slice of some length
and add new slaves on empty positions. Then if a slice passed to function, that function will know
the lengt of the slice and can loop over it.



*/
