package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

type Slots struct {
	Slots []Slot `json:"slots"`
}

type Slot struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Idle        bool   `json:"idle"`
}

type Units struct {
	Units []Unit `json:"units"`
}

type Unit struct {
	ID             string `json:"id"`
	State          string `json:"state"`
	Error          string `json:"error"`
	Project        int    `json:"project"`
	Run            int    `json:"run"`
	Clone          int    `json:"clone"`
	Gen            int    `json:"gen"`
	Core           string `json:"core"`
	Unit           string `json:"unit"`
	Percentdone    string `json:"percentdone"`
	Eta            string `json:"eta"`
	PPD            string `json:"ppd"`
	CreditEstimate string `json:"creditestimate"`
	WaitingOn      string `json:"waitingon"`
	NextAttempt    string `json:"nextattempt"`
	TimeRemaining  string `json:"timeremaining"`
	TotalFrames    int    `json:"totalframes"`
	FramesDone     int    `json:"framesdone"`
	Assigned       string `json:"assigned"`
	Timeout        string `json:"timeout"`
	Deadline       string `json:"deadline"`
	WS             string `json:"ws"`
	CS             string `json:"cs"`
	Attempts       int    `json:"attempts"`
	Slot           string `json:"slot"`
	TPF            string `json:"tpf"`
	BaseCredit     string `json:"basecredit"`
}

var slotinfo = "slot-info"
var queueinfo = "queue-info"

func (client *FAHClient) flavor() string {
	return "FAH"
}

//
// connectFahClient
//
// Open a socket to the FAH Client given in parameter
//

func (client *FAHClient) connect() error {
	var err error = nil

	if client.Ip == "" || client.Port < 1024 {
		return fmt.Errorf("invalid parameter for #{client.style()}")
	}

	adr := fmt.Sprintf("%s:%d", client.Ip, client.Port)

	// if we have no connection, then try to connect
	if client.connection != nil {
		return err
	}

	fmt.Printf("open connection to %s\n", adr)
	client.connection, err = net.DialTimeout("tcp", adr, 10*time.Second)

	if err != nil {
		client.ConnectionError = err
		return err
	}

	_ = client.receive(nil) // read the banner from the FAH Client

	authMsg := fmt.Sprintf("auth %s\n", client.Pwd)
	err = client.send(authMsg)
	if err == nil {
		_ = client.receive(nil)
	}
	if err != nil {
		err = client.disconnect()
	}

	return err
}

func (client *FAHClient) getConnection() *net.Conn {
	return &client.connection
}

func (client *FAHClient) isConnected() bool {
	return client.connection != nil
}

func (client *FAHClient) disconnect() error {
	err := client.connection.Close()
	client.connection = nil
	client.ConnectionError = err
	return err
}

//
// sendBoincClient
// Parameter:	client	management object for the connected client
//				object 	what data object will be send
// Result:		error 	error information or nil in case of success
//
func (client *FAHClient) send(object interface{}) error {
	if client.Debug == true {
		fmt.Printf("%s", object)
	}
	_, err := fmt.Fprintf(client.connection, "%s\n", object)
	return err
}

//
// method receive
// Parameter:	client	management object for the connected client
//				object  data object will be received
// Result:		none
//
func (client *FAHClient) receive(object interface{}) error {
	message, _ := bufio.NewReader(client.connection).ReadString('>')

	msg := PyPON2JSON(message)

	if client.Debug == true {
		_, _ = fmt.Printf("%q\n", msg)
	}
	var err error = nil

	if object != nil {
		err = json.Unmarshal([]byte(msg), object)
		if err != nil {
			err = fmt.Errorf("Error unmarshaling: %v\n", err)
		}
	}

	return err
}

//
// awful try to make the unknown PyON into real JSON for parsing
// e.g. Replace True with true, False with false and others
//

func PyPON2JSON(message string) string {
	msg := strings.Replace(message, "True", "true", -1)
	msg = strings.Replace(msg, "False", "false", -1)
	msg = strings.Replace(msg, "PyON 1 slots", "\"slots\":", -1)
	msg = strings.Replace(msg, "PyON 1 units", "\"units\":", -1)
	msg = strings.Replace(msg, "\n---\n>", "", -1)
	msg = strings.Replace(msg, "\\n", "\n", -1)

	msg = fmt.Sprintf("{\n%s\n}", msg)
	return msg
}

//
//
//
func (client *FAHClient) loadState() {

	for true {
		if client.connection == nil {
			return
		}

		err := client.send(slotinfo)
		if err == nil {
			_ = client.receive(&client.Slots)
		}

		err = client.send(queueinfo)
		if err == nil {
			_ = client.receive(&client.Units)
		}

		time.Sleep(time.Duration(client.Refresh) * time.Second)

	}
}

//
// loadFahStats
//
// This function start for each client a background process to poll for status
//
func loadFahStats() {

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
					go client.loadState()
				}
			}
		}
		// wait a period of time and try the client list again to connect those not yet connected
		time.Sleep(20 * time.Second)
	}
}
