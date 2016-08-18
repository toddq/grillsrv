package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// UR[2 Byte Grill Temp][2 Byte food probe Temp][2 Byte Target Temp]
// [skip 22 bytes][2 Byte target food probe][1byte on/off/fan][5 byte tail]
const (
	grillTemp        = 2
	probeTemp        = 4
	grillSetTemp     = 6
	curveRemainTime  = 20
	warnCode         = 24
	probeSetTemp     = 28
	grillState       = 30
	grillMode        = 31
	fireState        = 32
	fileStatePercent = 33
	profileEnd       = 34
	grillType        = 35

/*
int var2 = PubFunction.ByteToInt(var1, 2, 4);
int var3 = PubFunction.ByteToInt(var1, 4, 6);
int var4 = PubFunction.ByteToInt(var1, 6, 8);
int var5 = PubFunction.ByteToInt(var1, 20, 24);
int var6 = PubFunction.ByteToInt(var1, 24, 28);
int var7 = PubFunction.ByteToInt(var1, 28, 30);
int var8 = PubFunction.ByteToInt(var1, 30, 31);
int var9 = PubFunction.ByteToInt(var1, 31, 32);
int var10 = PubFunction.ByteToInt(var1, 32, 33);
int var11 = PubFunction.ByteToInt(var1, 33, 34);
int var12 = PubFunction.ByteToInt(var1, 34, 35);
int var13 = PubFunction.ByteToInt(var1, 35, 36);
DataManager.getInstance().mGrillTemperature = var2;
DataManager.getInstance().mFoodTemperature = var3;
DataManager.getInstance().mSetTemperature = var4;
DataManager.getInstance().mCurveRemainTime.SetTime(var5);
DataManager.getInstance().mWarnCode = var6;
DataManager.getInstance().mGrillState = var8;
DataManager.getInstance().mGrillMode = var9;
DataManager.getInstance().mNetworkSetFoodTemperature = var7;
DataManager.getInstance().mFireState = var10;
DataManager.getInstance().mFileStatePercent = var11;
DataManager.getInstance().mProfileEnd = var12;
if(DataManager.getInstance().mGrillMode == 0) {
	 DataManager.getInstance().mProfileSelID = -1;
	 DataManager.getInstance().mProfileStep = 0;
}
*/
)

type payload struct {
	Cmd    string `json:"cmd"`
	Params string `json:"params"`
}

// Grill ...
// struct
type Grill struct {
	grillIP     string
	ExternalIP  string `json:"ip"`
	serial      string
	ssid        string
	password    string
	ssidlen     int
	passwordlen int
	serverip    string
	port        string
	serveriplen int
	portlen     int
}

var grillStates = map[int]string{
	0: "OFF",
	1: "ON",
	2: "FAN",
	3: "REMAIN",
}
var fireStates = map[int]string{
	0: "DEFAULT",
	1: "OFF",
	2: "STARTUP",
	3: "RUNNING",
	4: "COOLDOWN",
	5: "FAIL",
}

var myGrill = Grill{
	grillIP: "LAN_IP:PORT",
	//grillIP:  "FQDN:PORT",
	serial:   "GMGSERIAL",
	ssid:     "SSID",
	password: "WIFI_PASS",
	serverip: "52.26.201.234",
	port:     "8060",
}

func main() {
	var buf bytes.Buffer
	myGrill.ssidlen = len(myGrill.ssid)
	myGrill.passwordlen = len(myGrill.password)
	myGrill.serveriplen = len(myGrill.serverip)
	myGrill.portlen = len(myGrill.port)

	http.HandleFunc("/temp", allTemp)                // all temps GET UR001!
	http.HandleFunc("/temp/grill", singleTemp)       // grill temp GET
	http.HandleFunc("/temp/probe", singleTemp)       // probe temp GET
	http.HandleFunc("/temp/grilltarget", singleTemp) // grill target temp GET/POST UT00!
	http.HandleFunc("/temp/foodtarget", singleTemp)  // food target temp GET/POST UF00!
	http.HandleFunc("/power", power)                 // power POST on/off UK001!/UK004!
	http.HandleFunc("/id", id)                       // grill id GET UL!
	http.HandleFunc("/info", info)                   // all fields GET UL!
	http.HandleFunc("/firmware", firmware)           // firmware GET UN!
	http.HandleFunc("/cmd", cmd)                     // cmd POST

	http.HandleFunc("/",
		func(w http.ResponseWriter, req *http.Request) {
			requestedFile := req.URL.Path[1:]
			switch requestedFile {
			case "usewifi":
				ptp := false
				iface, err := net.InterfaceAddrs()
				if err != nil {
					println(err.Error())
					os.Exit(1)
				}
				for _, ip := range iface {
					if strings.Contains(ip.String(), "192.168.16") {
						ptp = true
					}
				}
				if ptp {
					myGrill.grillIP = "192.168.16.254"
					fmt.Println("Message: PTP to Wifi")
					fmt.Fprintf(&buf, "UH%c%c%s%c%s!", 0, myGrill.ssidlen, myGrill.ssid, myGrill.passwordlen, myGrill.password)
				} else {
					fmt.Println("Need to be connected Ptp to send this message")
				}
			case "servermode":
				fmt.Println("Message: Wifi to Server Mode")
				fmt.Fprintf(&buf, "UG%c%s%c%s%c%s%c%s!", myGrill.ssidlen, myGrill.ssid, myGrill.passwordlen, myGrill.password, myGrill.serveriplen, myGrill.serverip, myGrill.portlen, myGrill.port)
			case "serverkey":
				fmt.Println("Message: Create Server Key")
				// curl 'https://api.ipify.org?format=json'
				r, err := http.Get("https://api.ipify.org?format=json")
				if err != nil {
					fmt.Println(err.Error())
				}
				defer r.Body.Close()
				err = json.NewDecoder(r.Body).Decode(&myGrill)
				serverKey := []byte(fmt.Sprint(myGrill.serial, myGrill.ExternalIP))
				fmt.Println("Serial:", myGrill.serial)
				fmt.Println("IP:", myGrill.ExternalIP)
				fmt.Println("ServerKey Bytes:", serverKey)
				fmt.Println("ServerKey:", fmt.Sprint(myGrill.serial, myGrill.ExternalIP))
			case "grillinfo":
				fmt.Println("Message: Get Grill Temps?")
				fmt.Fprint(&buf, "URCV!")
			case "externalip":
				fmt.Println("Message: Get External IP")
				fmt.Fprint(&buf, "GMGIP!")
			default:
				w.WriteHeader(404)
			}
		})

	http.ListenAndServe(":8000", nil)
}
func singleTemp(w http.ResponseWriter, req *http.Request) {
	//fmt.Printf("%s\n", req.Method)
	if req.Method == "GET" {
		requestedTemp := req.URL.Path[6:]
		var buf bytes.Buffer
		fmt.Println("Message: Get Info")
		fmt.Fprint(&buf, "UR001!")
		grillResponse, err := sendData(&buf)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
			return
		}
		var writebuf bytes.Buffer
		fmt.Fprint(&writebuf, "{ ")
		switch requestedTemp {
		case "grill":
			fmt.Fprintf(&writebuf, "\"grilltemp\" : %v ", grillResponse[grillTemp])
		case "grilltarget":
			fmt.Fprintf(&writebuf, "\"grillsettemp\" : %v ", grillResponse[grillSetTemp])
		case "probe":
			fmt.Fprintf(&writebuf, "\"probetemp\" : %v ", grillResponse[probeTemp])
		case "probetarget":
			fmt.Fprintf(&writebuf, "\"probesettemp\" : %v ", grillResponse[probeSetTemp])
		}
		fmt.Fprint(&writebuf, " }")
		w.Write(writebuf.Bytes())
	} else if req.Method == "POST" {
		/*
			TODO
			 int var51 = var50 % 10;
			 int var52 = var50 % 100 / 10;
			 int var53 = var50 / 100;
		*/
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), 405)
		return
	}
}

func allTemp(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	fmt.Println("Message: Get Info")
	fmt.Fprint(&buf, "UR001!")
	grillResponse, err := sendData(&buf)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	var writebuf bytes.Buffer
	fmt.Fprint(&writebuf, "{ ")
	fmt.Fprintf(&writebuf, "\"grilltemp\" : %v , ", grillResponse[grillTemp])
	fmt.Fprintf(&writebuf, "\"grillsettemp\" : %v , ", grillResponse[grillSetTemp])
	fmt.Fprintf(&writebuf, "\"probetemp\" : %v , ", grillResponse[probeTemp])
	fmt.Fprintf(&writebuf, "\"probesettemp\" : %v", grillResponse[probeSetTemp])
	fmt.Fprint(&writebuf, " }")
	w.Write(writebuf.Bytes())
}

func id(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	fmt.Println("Message: Get Grill Id")
	fmt.Fprint(&buf, "UL!")
	grillResponse, err := sendData(&buf)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	w.Write(grillResponse)
}

func firmware(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	fmt.Println("Message: Get Firmware")
	fmt.Fprint(&buf, "UN!")
	grillResponse, err := sendData(&buf)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	w.Write(grillResponse)
}

func cmd(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	// change broadcast to client
	// ptp to Wifi
	// server mode
	//fmt.Println("Message: Run Command")
	if req.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), 405)
		return
	}
	defer req.Body.Close()
	var pay payload
	err := json.NewDecoder(req.Body).Decode(&pay)
	fmt.Printf("Decoded Request: %s %s %s\n", &pay, pay.Cmd, pay.Params)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	switch pay.Cmd {
	case "btoc":
		fmt.Fprintf(&buf, "UH%c%c%s%c%s!", 0, myGrill.ssidlen, myGrill.ssid, myGrill.passwordlen, myGrill.password)
	}
	grillResponse, err := sendData(&buf)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	w.Write(grillResponse)
}

func info(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	fmt.Println("Message: Get Info")
	fmt.Fprint(&buf, "UR001!")
	grillResponse, err := sendData(&buf)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	var writebuf bytes.Buffer
	// gmg support
	// UR[2 Byte Grill Temp][2 Byte food probe Temp][2 Byte Target Temp][skip 22 bytes][2 Byte target food probe][1byte on/off/fan][5 byte tail]
	fmt.Fprint(&writebuf, "{ ")
	fmt.Fprintf(&writebuf, "\"grilltemp\" : %v , ", grillResponse[grillTemp])
	fmt.Fprintf(&writebuf, "\"grillsettemp\" : %v , ", grillResponse[grillSetTemp])
	fmt.Fprintf(&writebuf, "\"probetemp\" : %v , ", grillResponse[probeTemp])
	fmt.Fprintf(&writebuf, "\"probesettemp\" : %v ,", grillResponse[probeSetTemp])
	fmt.Fprintf(&writebuf, "\"curveremaintime\" : %v ,", grillResponse[curveRemainTime])
	fmt.Fprintf(&writebuf, "\"warncode\" : %v ,", grillResponse[warnCode])
	fmt.Fprintf(&writebuf, "\"grillstate\" : \"%s\" ,", grillStates[int(grillResponse[grillState])])
	fmt.Fprintf(&writebuf, "\"firestate\" : \"%s\" ,", fireStates[int(grillResponse[fireState])])
	fmt.Fprintf(&writebuf, "\"filestatepercent\" : %v, ", grillResponse[fileStatePercent])
	fmt.Fprintf(&writebuf, "\"profileend\" : %v, ", grillResponse[profileEnd])
	fmt.Fprintf(&writebuf, "\"grilltype\" : %v", grillResponse[grillType])
	fmt.Fprint(&writebuf, " }")
	w.Write(writebuf.Bytes())
}
func power(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	if req.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), 405)
		return
	}
	defer req.Body.Close()
	var pay payload
	err := json.NewDecoder(req.Body).Decode(&pay)
	fmt.Printf("Decoded Request: %s %s %s\n", &pay, pay.Cmd, pay.Params)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	switch pay.Cmd {
	case "on":
		fmt.Fprint(&buf, "UK001!")
	case "off":
		fmt.Fprint(&buf, "UK004!")
	}
	grillResponse, err := sendData(&buf)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("{ \"error\": \"%s\" }", err.Error())))
		return
	}
	w.Write(grillResponse)
}

func sendData(b *bytes.Buffer) ([]byte, error) {
	//b = []byte("UWFM!") // leaveServerMode
	if b.Len() == 0 {
		return nil, errors.New("Nothing to Send to Grill")
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s", myGrill.grillIP), 3*time.Second)
	timeout := time.Now().Add(3 * time.Second)
	conn.SetReadDeadline(timeout)
	if err != nil {
		return nil, errors.New("Connection to Grill Failed")
	}
	fmt.Println("Connected")

	defer conn.Close()
	fmt.Println("Sending Data..")
	ret, err := conn.Write(b.Bytes())
	if err != nil {
		return nil, errors.New("Failure Sending Payload to Grill")
	}
	fmt.Printf("Bytes Written: %v\n", ret)
	b.Reset()

	fmt.Println("Reading Data..")
	barray := make([]byte, 1024)
	status, err := bufio.NewReader(conn).Read(barray)
	if err != nil {
		return nil, errors.New("Failed Reading Result From Grill")
	}
	// trim null of 1024 byte array
	//barray = bytes.Trim(barray, "\x00")
	barray = barray[:36]

	// print what we got back
	fmt.Println(string(b.Bytes()))
	fmt.Println(string(barray))
	fmt.Println(barray)
	fmt.Println("Bytes Read:", status)
	fmt.Println("Read Buffer Size:", len(barray))
	return barray, nil
}
