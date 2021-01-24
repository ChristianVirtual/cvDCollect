package main

import (
	"fmt"
	"net"
	"time"
)

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
		client.connection, err = net.DialTimeout("tcp", adr, 5*time.Second)

		if err == nil {
			client.stopLoop = false
		} else {
			client.connection = nil
			client.connectionError = err
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

		time.Sleep(10 * time.Second)

		if client.stopLoop == true {
			break
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
		time.Sleep(60 * time.Second)
	}
}
