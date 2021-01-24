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

//
// sendBoincClient
// Parameter:	client	management object for the connected client
//				object 	what data object will be send
// Result:		error 	error information or nil in case of success
//
func sendFahClient(client *FAHClient, cmd string) error {
	if client.Debug == true {
		fmt.Printf("send: %s\n", cmd)
	}
	fmt.Fprintf(client.connection, "%s\n", cmd)
	return nil
}

//
// recvBoincClient
// Parameter:	client	management object for the connected client
//				object  data object will be received
// Result:		none
//
func recvFahClient(client *FAHClient, object interface{}) {
	message, _ := bufio.NewReader(client.connection).ReadString('>')

	msg := PyPON2JSON(message)

	if client.Debug == true {
		_, _ = fmt.Printf("%q\n", msg)
	}

	if object != nil {
		err := json.Unmarshal([]byte(msg), object)
		if err != nil {
			_ = fmt.Errorf("Error unmarshaling: %v\n", err)
		}
	}
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
// connectFahClient
//
// Open a socket to the FAH Client given in parameter
//

func connectFahClient(client *FAHClient) {
	var err error

	if client.Ip == "" || client.Port < 1024 {
		return
	}

	adr := fmt.Sprintf("%s:%d", client.Ip, client.Port)

	// if we have no connection, then try to connect
	if client.connection == nil {
		fmt.Printf("open connection to %s\n", adr)
		client.connection, err = net.DialTimeout("tcp", adr, 10*time.Second)

		recvFahClient(client, nil)

		client.stopLoop = false

		authMsg := fmt.Sprintf("auth %s\n", client.Pwd)
		err = sendFahClient(client, authMsg)
		if err == nil {
			recvFahClient(client, nil)
		} else {
			client.connection = nil
			client.ConnectionError = err
			fmt.Printf("error: %s\n", err)
		}
	}
}

//
//
//
func loadFahStatusForClient(client *FAHClient) {

	client.stopLoop = true

	for true {
		if client.connection == nil {
			return
		}

		err := sendFahClient(client, "slot-info\n")
		if err == nil {
			recvFahClient(client, &client.Slots)
		} else {
			client.stopLoop = true
		}

		err = sendFahClient(client, "queue-info\n")
		if err == nil {
			recvFahClient(client, &client.Units)
		} else {
			client.stopLoop = true
		}

		time.Sleep(time.Duration(client.Refresh) * time.Second)

		if client.stopLoop == true {
			_ = client.connection.Close()
			client.connection = nil
			client.stopLoop = false
			return
		}
	}
}

//
// loadFahStats
//
// This function start for each client a background process to poll for status
//
func loadFahStats() {

	// loop forever (in background)
	for true {
		// go over the list of clients
		for idx := range dcClients.FAHConfig.Clients {
			// get the reference
			var client = &dcClients.FAHConfig.Clients[idx]
			// if we have no connection yet
			if client.connection == nil {
				// then open the connection
				connectFahClient(client)
				fmt.Printf("client %s (%s)\n", client.Name, client.Ip)

				if client.connection != nil {
					go loadFahStatusForClient(client)
				}
			}
		}
		// wait a minute and try the client list again
		time.Sleep(20 * time.Second)
	}
}
