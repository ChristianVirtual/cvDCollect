package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"time"
)

//
// DCClients
//
// Structure with two array for the FAH and BOINC clients
//

type DCClients struct {
	ServerPort  int         `json:"port"`
	BOINCConfig BOINCConfig `json:"boinc"`
	FAHConfig   FAHConfig   `json:"fah"`
	// internal updated attributes
	BoincWUList []BoincWUReference
}

//
// make a DCConfig interface
//
type DCConfig interface {
}

type BOINCConfig struct {
	BoincCmd string        `json:"boinccmd"`
	Refresh  float64       `json:"refresh"`
	Clients  []BoincClient `json:"clients"`
}

type FAHConfig struct {
	Refresh float64     `json:"refresh"`
	Clients []FAHClient `json:"clients"`
}

//
// make a Client interface
//

type Clients []Client
type Client interface {
	flavor()     // ask for what flavor this client is for (FAH or BOINC)
	connect()    // connect to the client
	send()       // send a command to the client
	receive()    // receive data from the client
	disconnect() // disconnect from the client
	isConnected()
}

//
// DCClient
//
// Common definitions for all DC clients; used via "faked" inheritance
//
type DCClient struct {
	Name    string `json:"name"`
	Ip      string `json:"ip"`
	Port    int    `json:"port"`
	Pwd     string `json:"pwd"`
	Debug   bool   `json:"debug"`
	Refresh int8   `json:"refresh"`

	connection      net.Conn
	ConnectionError error
}

//
// BoincClient
//
// Structure to store the values relevant to manage one BOINC client
//
type BoincClient struct {
	DCClient         // "fake" inheritance
	ClientStateReply ClientStateReply
}

type BoincWUReference struct {
	Client string
	WUName string
}

//
// FahClient
//
// Structure to store the values relevant to manage one FAH client
//
type FAHClient struct {
	DCClient // "fake" inheritance
	Slots    Slots
	Units    Units
}

//
// Global list for all DC clients
var dcClients DCClients

func loadStats(clients *Clients) {
	// loop forever (in background) and fetch disconnected clients for reconnect
	for true {
		// go over the list of clients
		for idx := range dcClients.FAHConfig.Clients {
			// get the reference
			var client = &dcClients.FAHConfig.Clients[idx]
			// if we have no connection yet
			if client.isConnected() == false {
				// then open the connection
				client.connect()
				fmt.Printf("%s client %s (%s)\n", client.flavor(), client.Name, client.Ip)

				// and if successful start loading the data in background
				if client.isConnected() == true {
					go func() {
						client.loadState()
					}()
				}
			}
		}
		// wait a period of time and try the client list again to connect those not yet connected
		time.Sleep(20 * time.Second)
	}
}

//
// boincHandler URL handler
//
func boincHandler(w http.ResponseWriter, _ *http.Request) {
	//	clientName := r.URL.Path[len("/boinc/"):]

	outputDefaultHeader(w)

	clienttemplate, err := template.ParseFiles("html/cvDCollector_boinc.html")
	if err != nil {
		log.Print(err)
	}

	dcClients.BoincWUList = nil
	for _, client := range dcClients.BOINCConfig.Clients {
		for _, result := range client.ClientStateReply.ClientState.Results {
			dcClients.BoincWUList = append(dcClients.BoincWUList, BoincWUReference{Client: client.Name, WUName: result.WUName})
		}
	}

	sort.Slice(dcClients.BoincWUList, func(i, j int) bool {
		return dcClients.BoincWUList[i].WUName < dcClients.BoincWUList[j].WUName
	})

	WUmin := "?"
	WUmax := "?"
	lenList := len(dcClients.BoincWUList)
	if lenList > 0 {
		WUmin = dcClients.BoincWUList[0].WUName
		WUmax = dcClients.BoincWUList[lenList-1].WUName
	}
	data := struct {
		WUMin        string
		WUMax        string
		BoincClients []BoincClient
	}{
		WUMin:        WUmin,
		WUMax:        WUmax,
		BoincClients: dcClients.BOINCConfig.Clients,
	}

	err = clienttemplate.Execute(w, data)
	if err != nil {
		_, _ = fmt.Printf("error %s", err)
	}
}

//
// fahHandler URL handler
//
func fahHandler(w http.ResponseWriter, _ *http.Request) {
	//	clientName := r.URL.Path[len("/fah/"):]

	outputDefaultHeader(w)

	clienttmp, err := template.ParseFiles("html/cvDCollector_fah.html")
	if err != nil {
		log.Print(err)
	}

	data := struct {
		FAHClients []FAHClient
	}{
		FAHClients: dcClients.FAHConfig.Clients,
	}

	err = clienttmp.Execute(w, data)
	if err != nil {
		_, _ = fmt.Printf("error %s", err)
	}
}

//
// updateHandler URL handler
//
func updateHandler(w http.ResponseWriter, r *http.Request) {
	// clientName := r.URL.Path[len("/update/"):]

	if err := r.ParseForm(); err != nil {
		fmt.Printf("%s\n", err)
		return
	}

	clientName, err := url.QueryUnescape(r.Form.Get("client"))
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}

	switch r.Method {
	case "POST":
		for idx := range dcClients.BOINCConfig.Clients {
			var client = &dcClients.BOINCConfig.Clients[idx]
			if clientName == client.Name || clientName == "all" {

				fmt.Printf("trigger update for %s (%s)\n", client.Name, client.Ip)
				reqBody, err := ioutil.ReadAll(r.Body)
				if err != nil {
					log.Fatal(err)
				}

				cmd := exec.Command("boinccmd", "--host", client.Ip, "--passwd", client.Pwd, "--project", "http://www.worldcommunitygrid.org", "update")
				if err := cmd.Run(); err != nil {
					fmt.Println("Error: ", err)
				}

				fmt.Printf("%s\n", reqBody)
				_, _ = fmt.Fprintf(w, "Received a POST request to update %s\n", client.Name)
			}
		}
	default:
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, "%s", http.StatusText(http.StatusNotImplemented))

	}
}

//
// reloadHandler URL handler
//
func reloadHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/reload/"):]

	outputDefaultHeader(w)
	if title == "boinc" || title == "all" {
		for idx := range dcClients.BOINCConfig.Clients {
			var client = &dcClients.BOINCConfig.Clients[idx]

			if client.connection != nil {
				if err := client.connection.Close(); err != nil {
					fmt.Printf("client %s (%s): %s\n", client.Name, client.Ip, err)
				}
			}

			if err := client.connect(); err != nil {
				fmt.Printf("reload client %s (%s), error: %s\n", client.Name, client.Ip, err)
			}
		}
	}

	for _, client := range dcClients.BOINCConfig.Clients {
		_, _ = fmt.Fprintf(w, "<h2>%s</h2>", client.Name)

		if client.ConnectionError != nil {
			_, _ = fmt.Fprintf(w, "error=%s<br>", client.ConnectionError)
		}
	}
}

//
//
//
func outputDefaultHeader(w http.ResponseWriter) {

	w.Header().Set("cache-control", "no-cache, must-revalidate, post-check=0, pre-check=0, max-age=0")
	w.Header().Set("expires", "0")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("viewport", "width=device-width, initial-scale=1")
	//w.Header().Set("", "")
	//w.Header().Set("", "")
	//w.Header().Set("", "")
	//w.Header().Set("", "")
	//w.Header().Set("", "")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	//	_, _ = fmt.Fprintf(w, "")
}

//
// load the config file for the remote clients
//
func loadConfig() {
	//
	// load the JSON file with clients and password
	//
	jsonFile, err := os.Open("clients.json")
	if err != nil {
		// os.Open has an error ?
		fmt.Println(err)
		os.Exit(-1)
	}

	defer jsonFile.Close() // whenever, close the file

	// process the content of the config file
	byteValue, _ := ioutil.ReadAll(jsonFile)
	if err := json.Unmarshal(byteValue, &dcClients); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}

//
// main
//
func main() {

	//
	// start network connection for each client
	//
	loadConfig()

	fmt.Printf("%d FAH clients in list\n", len(dcClients.FAHConfig.Clients))
	go func() {
		loadFahStats()
	}()
	//go loadStats(&dcClients.FAHConfig.Clients)
	//	loadClientsState(&dcClients.FAHConfig.Clients)

	fmt.Printf("%d BOINC clients in list\n", len(dcClients.BOINCConfig.Clients))
	go func() {
		loadBoincStats()
	}()

	//	loadClientsState(&dcClients.BOINCConfig.Clients)

	fscss := http.FileServer(http.Dir("css"))
	http.Handle("/css/", http.StripPrefix("/css/", fscss))
	fshtml := http.FileServer(http.Dir("html"))
	http.Handle("/html/", http.StripPrefix("/html/", fshtml))
	fsjs := http.FileServer(http.Dir("js"))
	http.Handle("/js/", http.StripPrefix("/js/", fsjs))
	fsimg := http.FileServer(http.Dir("image"))
	http.Handle("/image/", http.StripPrefix("/image/", fsimg))

	// establish the various handlers
	http.HandleFunc("/boinc/", boincHandler)   // refresh clients
	http.HandleFunc("/fah/", fahHandler)       // refresh clients
	http.HandleFunc("/update", updateHandler)  // update API via POST
	http.HandleFunc("/reload/", reloadHandler) // reload overall config and restart communication

	// start the web server
	addr := fmt.Sprintf(":%d", dcClients.ServerPort)
	log.Fatal(http.ListenAndServe(addr, nil))
}
