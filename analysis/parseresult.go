package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"time"
)

type Counters struct {
	InProgress  int `json:"connection:inprogress"`
	Established int `json:"connection:established"`
	Error       int `json:"error"`
	Success     int `json:"success"`
	Send        int `json:"message:sent"`
	Recv        int `json:"message:received"`
	SendSize    int `json:"message:recvSize"`
	RecvSize    int `json:"message:sendSize"`
	LT_100      int `json:"message:lt:100"`
	LT_200      int `json:"message:lt:200"`
	LT_300      int `json:"message:lt:300"`
	LT_400      int `json:"message:lt:400"`
	LT_500      int `json:"message:lt:500"`
	LT_600      int `json:"message:lt:600"`
	LT_700      int `json:"message:lt:700"`
	LT_800      int `json:"message:lt:800"`
	LT_900      int `json:"message:lt:900"`
	LT_1000     int `json:"message:lt:1000"`
	GE_1000     int `json:"message:ge:1000"`
}

type Monitor struct {
	Timestamp string `json:"Time"`
	Counters  Counters
}

func main() {
	var infile = flag.String("input", "", "Specify the input file")
	var lastLatency bool
	lastLatency = false
	var all bool
	all = false
	var rate bool
	rate = false
	var sizerate bool
	sizerate = false
	flag.BoolVar(&all, "all", false, "Print all information")
	flag.BoolVar(&lastLatency, "lastlatency", false, "Print the last item of latency")
	flag.BoolVar(&rate, "rate", false, "Print send/recv rate")
	flag.BoolVar(&sizerate, "sizerate", false, "Print send/recv size rate")

	flag.Usage = func() {
		fmt.Println("-input <input_file> : specify the input file")
		fmt.Println("-lastlatency        : print the last item of latency")
		fmt.Println("-all		 : print all information")
		fmt.Println("-rate               : print send/recv rate")
		fmt.Println("-sizerate           : print send/recv message size rate")
	}
	flag.Parse()
	if infile == nil || *infile == "" {
		fmt.Println("No input")
		flag.Usage()
		return
	}
	raw, err := ioutil.ReadFile(*infile)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	var monitors []Monitor
	json.Unmarshal(raw, &monitors)
	if all {
		for _, v := range monitors {
			// timestamp succ err inprogress send recv sendSize recvSize lt_100 lt_200 lt_300 lt_400 lt_500 lt_600 lt_700 lt_800 lt_900 lt_1000 ge_1000
			fmt.Printf("%s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d\n",
				v.Timestamp, v.Counters.Established, v.Counters.Error, v.Counters.InProgress,
				v.Counters.Send,
				v.Counters.Recv,
				v.Counters.SendSize,
				v.Counters.RecvSize,
				v.Counters.LT_100,
				v.Counters.LT_200,
				v.Counters.LT_300,
				v.Counters.LT_400,
				v.Counters.LT_500,
				v.Counters.LT_600,
				v.Counters.LT_700,
				v.Counters.LT_800,
				v.Counters.LT_900,
				v.Counters.LT_1000,
				v.Counters.GE_1000)
		}
	}
	if lastLatency {
		var v Monitor
		v = monitors[len(monitors)-1]
		fmt.Printf("['Latency category', 'Counters'],\n")
		fmt.Printf("['LT100', %d],\n", v.Counters.LT_100)
		fmt.Printf("['LT200', %d],\n", v.Counters.LT_200)
		fmt.Printf("['LT300', %d],\n", v.Counters.LT_300)
		fmt.Printf("['LT400', %d],\n", v.Counters.LT_400)
		fmt.Printf("['LT500', %d],\n", v.Counters.LT_500)
		fmt.Printf("['LT600', %d],\n", v.Counters.LT_600)
		fmt.Printf("['LT700', %d],\n", v.Counters.LT_700)
		fmt.Printf("['LT800', %d],\n", v.Counters.LT_800)
		fmt.Printf("['LT900', %d],\n", v.Counters.LT_900)
		fmt.Printf("['LT1000', %d],\n", v.Counters.LT_1000)
		fmt.Printf("['GE1000', %d],\n", v.Counters.GE_1000)
	}
	if rate {
		fmt.Printf("\tdata.addColumn('timeofday', 'Time');\n")
		fmt.Printf("\tdata.addColumn('number', 'Send count rate');\n")
		fmt.Printf("\tdata.addColumn('number', 'Recv count rate');\n")
		fmt.Printf("\tdata.addRows([\n")
		for i, j := 0, 1; j < len(monitors); i, j = i+1, j+1 {
			t1, _ := time.Parse(time.RFC3339, monitors[j].Timestamp)
			fmt.Printf("\t [[%d, %d, %d], %d, %d],\n", t1.Hour(), t1.Minute(), t1.Second(), monitors[j].Counters.Send - monitors[i].Counters.Send, monitors[j].Counters.Recv - monitors[i].Counters.Recv)
		}
		fmt.Printf("\t]);\n")
	}
	if sizerate {
		fmt.Printf("\tdata.addColumn('timeofday', 'Time');\n")
		fmt.Printf("\tdata.addColumn('number', 'Send message size (Byte/Sec)');\n")
		fmt.Printf("\tdata.addColumn('number', 'Recv message size (Byte/Sec)');\n")
		fmt.Printf("\tdata.addRows([\n")
		for i, j := 0, 1; j < len(monitors); i, j = i+1, j+1 {
			t1, _ := time.Parse(time.RFC3339, monitors[j].Timestamp)
			fmt.Printf("\t [[%d, %d, %d], %d, %d],\n", t1.Hour(), t1.Minute(), t1.Second(), monitors[j].Counters.SendSize - monitors[i].Counters.SendSize, monitors[j].Counters.RecvSize - monitors[i].Counters.RecvSize)
		}
		fmt.Printf("\t]);\n")
	}
}
