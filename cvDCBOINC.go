package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"net"
	"sort"
	"time"
)

//
// Structures for BOINC client authorization
//
// Basic information from here: https://boinc.berkeley.edu/trac/wiki/GuiRpcProtocol
// and observed real world responses
//

//
// BOINC Client Authentification
//
// first part of authorization with BOINC Client
type auth1 struct {
	XMLName xml.Name `xml:"boinc_gui_rpc_request"`
	Auth1   string   `xml:"auth1"`
}

// response from first part of authorization process; nonce used as salt
type nonce struct {
	XMLName xml.Name `xml:"boinc_gui_rpc_reply"`
	Nonce   string   `xml:"nonce"`
}

// salted password send back to BOINC client
// concatenate the nonce and the password (nonce first), then calculate the MD5 hash of the result, i.e: md5(nonce+password). Finally, send an <auth2> request with the calculated hash, in lowercase hexadecimal format.
type auth2 struct {
	XMLName   xml.Name `xml:"boinc_gui_rpc_request"`
	NonceHash string   `xml:"auth2>nonce_hash"`
}

//
// Project Status Structure for BOINC client
//
// Not all attributes are covered in this structure at is point in time
//
type Projects []Project
type Project struct {
	XMLName     xml.Name `xml:"project"`
	MasterUrl   string   `xml:"master_url"`
	ProjectName string   `xml:"project_name"`
	UserName    string   `xml:"user_name"`
	TeamName    string   `xml:"team_name"`
	HostVenue   string   `xml:"host_venue"`
	//EMailHash					string   `xml:"email_hash"`
	CrossProjectID  string  `xml:"cross_project_id"`
	ExternalCPID    string  `xml:"external_cpid"`
	CPIDTime        float64 `xml:"cpid_time"`
	UserTotalCredit float64 `xml:"user_total_credit"`
	UserAvgCredit   float64 `xml:"user_expavg_credit"`
	UserCreateTime  float64 `xml:"user_create_time"`
	HostTotalCredit float64 `xml:"host_total_credit"`
	HostAvgCredit   float64 `xml:"host_expavg_credit"`
	NJobsSuccess    int     `xml:"njobs_success"`
	NJobsError      int     `xml:"njobs_error"`
	ElapsedTime     float64 `xml:"elapsed_time"`
	// <gui_urls>
	//   <gui_url>
	//     <name>
	//     <description>
	//     <url>
	SchedPriority              float64 `xml:"sched_priority"`
	ProjectFilesDownloadedTime float64 `xml:"project_files_downloaded_time"`
	Venue                      string  `xml:"venue"`
	ProjectDir                 string  `xml:"project_dir"`
}

type Results []Result
type Result struct {
	XMLName                xml.Name   `xml:"result"`
	Name                   string     `xml:"name"`
	WUName                 string     `xml:"wu_name"`
	Platform               string     `xml:"platform"`
	VersionNum             string     `xml:"version_num"`
	ProjectUrl             string     `xml:"project_url"`
	FinalCPUTime           float64    `xml:"final_cpu_time"`
	FinalElapsedTime       float64    `xml:"final_elapsed_time"`
	ExitStatus             string     `xml:"exit_status"`
	State                  int        `xml:"state"`
	ReportDeadline         float64    `xml:"report_deadline"`
	ReceivedTime           float64    `xml:"received_time"`
	EstimatedTimeRemaining float64    `xml:"estimated_cpu_time_remaining"`
	Activetask             ActiveTask `xml:"active_task"`
	ReadyToReport          *struct{}  `xml:"ready_to_report"` // is nil when not ready, not nil when ready

	//
	// own attributed computed when loaded (e.g. convert timestamps to text)
	//
	IsFinished                     bool
	EstimatedTimeRemainingAsString string
}

type ActiveTask struct {
	XMLName           xml.Name `xml:"active_task"`
	TaskState         int      `xml:"active_task_state"`
	CheckpointCPUTime float64  `xml:"checkpoint_cpu_time"`
	ElapsedTime       float64  `xml:"elapsed_time"`
	WorkingSetSize    float64  `xml:"working_set_size"`
	ProgressRate      float64  `xml:"progress_rate"`
}

type App struct {
	XMLName          xml.Name `xml:"app"`
	Name             string   `xml:"name"`
	UserFriendlyName string   `xml:"user_friendly_name"`
	NonCpuIntensive  int      `xml:"non_cpu_intensive"`
}

type AppVersion struct {
	XMLName    xml.Name `xml:"app_version"`
	AppName    string   `xml:"app_name"`
	VersionNum int      `xml:"version_num"`
	Platform   string   `xml:"platform"`
	AvgNcpus   float64  `xml:"avg_ncpus"`
	Flops      float64  `xml:"flops"`
	APIVersion string   `xml:"api_version"`
}

type WorkUnit struct {
	XMLName        xml.Name `xml:"workunit"`
	Name           string   `xml:"name"`
	AppName        string   `xml:"app_name"`
	RscFpopsEst    float64  `xml:"rsc_fpops_est"`
	RscFpopsBound  float64  `xml:"rsc_fpops_bound"`
	RscMemoryBound float64  `xml:"rsc_memory_bound"`
	RscDiskBound   float64  `xml:"rsc_disk_bound"`
}

type HostInfo struct {
	XMLName        xml.Name `xml:"host_info"`
	Timezone       string   `xml:"timezone"`
	DomainName     string   `xml:"domain_name"`
	IPAddr         string   `xml:"ip_addr"`
	HostCPID       string   `xml:"host_cpid"`
	PnCPUs         int8     `xml:"p_ncpus"`
	PVendor        string   `xml:"p_vendor"`
	PModel         string   `xml:"p_model"`
	PFeatures      string   `xml:"p_features"`
	PFPOps         float64  `xml:"p_fpops"`
	PIOps          float64  `xml:"p_iops"`
	PMemBW         float64  `xml:"p_membw"`
	PCalculated    float64  `xml:"p_calculated"`
	PVMExtDisabled int8     `xml:"p_vm_extensions_disabled"`
	MNBytes        float64  `xml:"m_nbytes"`
	MCache         float64  `xml:"m_cache"`
	MSwap          float64  `xml:"m_swap"`
	DTotal         float64  `xml:"d_total"`
	DFree          float64  `xml:"d_free"`
	OSName         string   `xml:"os_name"`
	OSVersion      string   `xml:"os_version"`
	NUsableCoprocs int8     `xml:"n_usable_coprocs"`
	WslAvailable   int8     `xml:"wsl_available"`
	// coprocs
}

type NetStats struct {
	XMLName     xml.Name `xml:"net_stats"`
	BWUp        float64  `xml:"bwup"`
	AvgUp       float64  `xml:"avg_up"`
	AvgTimeUp   float64  `xml:"avg_time_up"`
	BWDown      float64  `xml:"bwdown"`
	AvgDown     float64  `xml:"avg_down"`
	AvgTimeDown float64  `xml:"avg_time_down"`
}

type TimeStats struct {
	XMLName                  xml.Name `xml:"time_stats"`
	OnFrac                   float64  `xml:"on_frac"`
	ConnectedFrac            float64  `xml:"connected_frac"`
	CpuNetworkAvailableFrac  float64  `xml:"cpu_and_network_available_frac"`
	ActiveFrac               float64  `xml:"active_frac"`
	GpuActiveFrac            float64  `xml:"gpu_active_frac"`
	ClientStartTime          float64  `xml:"client_start_time"`
	TotalStartTime           float64  `xml:"total_start_time"`
	TotalDuration            float64  `xml:"total_duration"`
	TotalActiveDuration      float64  `xml:"total_active_duration"`
	TotalGpuActiveDuration   float64  `xml:"total_gpu_active_duration"`
	Now                      float64  `xml:"now"`
	PreviousUptime           float64  `xml:"previous_uptime"`
	SessionActiveDuration    float64  `xml:"session_active_duration"`
	SessionGpuActiveDuration float64  `xml:"session_gpu_active_duration"`
}

//
// Contain reduced state of a client (projects and results)
//
type simpleGuiInfo struct {
	XMLName xml.Name `xml:"get_simple_gui_info"`
}

type simpleGuiInfoReply struct {
	XMLName       xml.Name `xml:"boinc_gui_rpc_reply"`
	SimpleGuiInfo struct {
		Projects Projects `xml:"project"`
		Results  Results  `xml:"result"`
	} `xml:"simple_gui_info"`
}

//
// Contain the entire state of a client
//
type GetState struct {
	XMLName xml.Name `xml:"get_state"`
}

type ClientStateReply struct {
	XMLName     xml.Name `xml:"boinc_gui_rpc_reply"`
	ClientState struct {
		HostInfo  HostInfo  `xml:"host_info"`
		NetStats  NetStats  `xml:"net_stats"`
		TimeStats TimeStats `xml:"time_stats"`
		// Coprocs
		Projects    []Project    `xml:"project"`
		Apps        []App        `xml:"app"`
		AppVersions []AppVersion `xml:"app_version"`
		WorkUnits   []WorkUnit   `xml:"workunit"`
		Results     Results      `xml:"result"`
	} `xml:"client_state"`
}

//
//
//
func (r Results) Len() int {
	return len(r)
}
func (r Results) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
func (r Results) Less(i, j int) bool {
	return (r[i].EstimatedTimeRemaining < r[j].EstimatedTimeRemaining) && (r[i].WUName < r[j].WUName)
}

//
// some constants to perform seconds to Day:Hour:Min:Sec conversion
const secDay = uint64(24 * 60 * 60)
const secHour = uint64(60 * 60)
const secMin = uint64(60)

//
// convertToDHMS
//
func convertToDHMS(secFloat float64) (day uint8, hour uint8, min uint8, sec uint8) {
	remSec := uint64(math.Round(secFloat))
	day = uint8(remSec / secDay)
	hour = uint8((remSec % secDay) / secHour)
	min = uint8((remSec % secHour) / secMin)
	sec = uint8(remSec % secMin)

	return day, hour, min, sec
}

//
// convertResultToDHMS
//
func convertResultToDHMS(result *Result) {
	days, hours, mins, secs := convertToDHMS(result.EstimatedTimeRemaining)
	if days == 0 {
		if hours == 0 {
			if mins == 0 {
				result.EstimatedTimeRemainingAsString = fmt.Sprintf("%ds", secs)
			} else {
				result.EstimatedTimeRemainingAsString = fmt.Sprintf("%dm:%ds", mins, secs)
			}
		} else {
			result.EstimatedTimeRemainingAsString = fmt.Sprintf("%dh:%dm:%ds", hours, mins, secs)
		}
	} else {
		result.EstimatedTimeRemainingAsString = fmt.Sprintf("%dd %dh:%dm:%ds", days, hours, mins, secs)
	}
}

//
//
//
func (client *BoincClient) flavor() string {
	return "BOINC"
}

//
// method connect()
// Parameter:	none
// Result:		error 	error information or nil in case of success
//

func (client *BoincClient) connect() error {
	var err error = nil

	if client.Ip == "" || client.Port < 1024 {
		return fmt.Errorf("invalid parameter for #{client.style()}")
	}

	adr := fmt.Sprintf("%s:%d", client.Ip, client.Port)

	// if we have no connection, then try to connect
	if client.connection != nil {
		return nil
	}

	if client.Refresh < 1 {
		client.Refresh = 10
	}
	fmt.Printf("open connection to %s\n", adr)
	client.connection, err = net.DialTimeout("tcp", adr, 5*time.Second)

	if err != nil {
		client.ConnectionError = err
		return err
	}

	passkey := client.Pwd
	authMsg := &auth1{}
	err = client.send(authMsg)
	if err == nil {
		nonceMsg := &nonce{}
		_ = client.receive(nonceMsg)
		password := nonceMsg.Nonce + passkey
		calculated := md5.Sum([]byte(password))
		var calculated2 = calculated[:]
		err = client.send(&auth2{NonceHash: hex.EncodeToString(calculated2)})
		if err == nil {
			_ = client.receive(nil)
		}
	}

	if err != nil {
		err = client.disconnect()
	}

	return err
}

func (client *BoincClient) getConnection() *net.Conn {
	return &client.connection
}

func (client *BoincClient) isConnected() bool {
	return client.connection != nil
}

func (client *BoincClient) disconnect() error {
	err := client.connection.Close()
	client.connection = nil
	client.ConnectionError = err
	return err
}

//
// method send
// Parameter:	object 	what data object will be send
// Result:		error 	error information or nil in case of success
//

func (client *BoincClient) send(object interface{}) error {
	enc, err := xml.MarshalIndent(object, "> ", "  ")
	if err != nil {
		_ = fmt.Errorf("Error marshaling: %v\n", err)
		return err
	} else {
		// append the delimiter at the end as asked by the BOINC definition
		enc2 := append(enc, 0x03)
		_, err := fmt.Fprintf(client.connection, "%s", enc2)
		if err != nil {
			_ = fmt.Errorf("Error writing data to client: %v\n", err)
		}
	}
	return err
}

//
// method receive
// Parameter:	object  data object will be received
// Result:		none
//
func (client *BoincClient) receive(object interface{}) error {
	message, _ := bufio.NewReader(client.connection).ReadString(0x03)
	var err error = nil
	if object != nil {
		if client.Debug == true {
			_, _ = fmt.Printf("%s\n", message)
		}
		err = xml.Unmarshal([]byte(message), object)
		if err != nil {
			err = fmt.Errorf("Error unmarshaling: %v\n", err)
		}
	}

	return err
}

//
// loadBoincStatusForClient
//
// loop for one BOINC client to load the actual state and fill internal structure
//
func (client *BoincClient) loadState() {
	for true {
		if client.connection == nil {
			return
		}

		state := GetState{}
		client.ClientStateReply = ClientStateReply{}
		err := client.send(&state)
		if err == nil {
			err = client.receive(&client.ClientStateReply)
			if err != nil {
				fmt.Printf("%s\n", err)
			}
			sort.Sort(client.ClientStateReply.ClientState.Results)

			for idx := range client.ClientStateReply.ClientState.Results {
				var result = &client.ClientStateReply.ClientState.Results[idx]
				// do some conversions once loaded
				convertResultToDHMS(result)
				result.IsFinished = result.EstimatedTimeRemaining == 0
			}
		}

		time.Sleep(time.Duration(client.Refresh) * time.Second)
	}
}

//
// loadBoincStats
//
// This function start for each client a background process to poll for status
//
func loadBoincStats() {

	// loop forever (in background) and fetch disconnected clients for reconnect
	for true {
		// go over the list of clients
		for idx := range dcClients.BOINCConfig.Clients {
			// get the reference
			var client = &dcClients.BOINCConfig.Clients[idx]
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
