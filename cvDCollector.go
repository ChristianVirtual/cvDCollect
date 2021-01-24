package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
)

//
// DCClients
//
// Structure with two array for the FAH and BOINC clients
//

type DCClients struct {
	Port        int         `json:"port"`
	BOINCConfig BOINCConfig `json:"boinc"`
	FAHConfig   FAHConfig   `json:"fah"`
	// internal updated attributes
	BoincWUList []BoincWUReference
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
// BoincClient
//
// Structure to store the values relevant to manage one BOINC client
//
type BoincClient struct {
	Name             string `json:"name"`
	Ip               string `json:"ip"`
	Port             int    `json:"port"`
	Pwd              string `json:"pwd"`
	Debug            bool   `json:"debug"`
	Refresh          int8   `json:"refresh"`
	stopLoop         bool
	connection       net.Conn
	ConnectionError  error
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
	Name            string `json:"name"`
	Ip              string `json:"ip"`
	Port            int    `json:"port"`
	Pwd             string `json:"pwd"`
	Debug           bool   `json:"debug"`
	Refresh         int8   `json:"refresh"`
	stopLoop        bool
	connection      net.Conn
	ConnectionError error
	Slots           Slots
	Units           Units
}

//
// Global list for all DC clients
var dcClients DCClients

//
// boincHandler URL handler
//
func boincHandler(w http.ResponseWriter, r *http.Request) {
	//	clientName := r.URL.Path[len("/boinc/"):]

	outputDefaultHeader(w, r)

	clienttmp, err := template.ParseFiles("html/cvDCollector_boinc.html")
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

	WUmin := ""
	WUmax := ""
	len := len(dcClients.BoincWUList)
	if len > 0 {
		WUmin = dcClients.BoincWUList[0].WUName
		WUmax = dcClients.BoincWUList[len-1].WUName
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

	err = clienttmp.Execute(w, data)
	if err != nil {
		_, _ = fmt.Printf("error %s", err)
	}
}

//
// fahHandler URL handler
//
func fahHandler(w http.ResponseWriter, r *http.Request) {
	//	clientName := r.URL.Path[len("/fah/"):]

	outputDefaultHeader(w, r)

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
	clientName := r.URL.Path[len("/update/"):]
	//p, _ := loadPage(title)

	clientName = r.FormValue("updateRequest")

	//	outputDefaultHeader(w, r)
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
	//p, _ := loadPage(title)

	outputDefaultHeader(w, r)

	if title == "boinc" || title == "all" {
		for idx := range dcClients.BOINCConfig.Clients {
			var client = &dcClients.BOINCConfig.Clients[idx]

			if client.connection != nil {
				err := client.connection.Close()
				if err != nil {
					fmt.Printf("client %s (%s): %s\n", client.Name, client.Ip, err)
				}
				client.connection = nil
			}

			connectBoincClient(client)
			fmt.Printf("client %s (%s)\n", client.Name, client.Ip)

			//			if client.connection != nil {
			//				go loadBoincStatusForClient(client)
			//			}
		}
	}

	for _, client := range dcClients.BOINCConfig.Clients {
		_, _ = fmt.Fprintf(w, "<h1>%s</h1>", client.Name)
		_, _ = fmt.Fprintf(w, "<div>")
		_, _ = fmt.Fprintf(w, "Results %d<br>", len(client.ClientStateReply.ClientState.Results))

		if client.ConnectionError != nil {
			_, _ = fmt.Fprintf(w, "error=%s<br>", client.ConnectionError)
		}
		sort.Sort(client.ClientStateReply.ClientState.Results)
		sort.Slice(client.ClientStateReply.ClientState.Results, func(i, j int) bool {
			return client.ClientStateReply.ClientState.Results[i].WUName < client.ClientStateReply.ClientState.Results[j].WUName
		})
	}
}

//
//
//
func outputDefaultHeader(w http.ResponseWriter, r *http.Request) {

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
	// os.Open has an error ?
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &dcClients)
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "%s\n", err)
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
	go loadFahStats()

	fmt.Printf("%d BOINC clients in list\n", len(dcClients.BOINCConfig.Clients))
	go loadBoincStats()

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
	http.HandleFunc("/update/", updateHandler) // update API via POST
	http.HandleFunc("/reload/", reloadHandler) // reload overall config and restart communication

	// start the web server
	addr := fmt.Sprintf(":%d", dcClients.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
