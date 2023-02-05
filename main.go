package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/spf13/pflag"
)

// Configuration
var CFG_LEFT_PORT string
var CFG_LEFT_PORT_LABEL string
var CFG_RIGHT_PORT string
var CFG_RIGHT_PORT_LABEL string
var CFG_READ_TIMEOUT int
var CFG_READ_BUFFER_SIZE int
var CFG_OUTPUT_FILE string
var CFG_LOG_CONTROL_FLOW bool

func init() {
	pflag.StringVarP(&CFG_LEFT_PORT, "left-port", "l", "COM1,19200,N,8,1", "Left port definition")
	pflag.StringVar(&CFG_LEFT_PORT_LABEL, "left-label", "Left Port", "an arbitrary label for the left port, used for better distinction in the logs")

	pflag.StringVarP(&CFG_RIGHT_PORT, "right-port", "r", "COM2,19200,N,8,1", "Right port definition")
	pflag.StringVar(&CFG_RIGHT_PORT_LABEL, "right-label", "Right Port", "an arbitrary label for the left port, used for better distinction in the logs")

	pflag.StringVarP(&CFG_OUTPUT_FILE, "output", "o", "", "log file. leave this emtpy to log to console only")

	pflag.IntVarP(&CFG_READ_TIMEOUT, "read-timeout", "t", 100, "Read timeout in ms. Adjust this to better detect packet boundaries")
	pflag.IntVar(&CFG_READ_BUFFER_SIZE, "read-bugger", 4096, "Read buffer size")
	pflag.BoolVar(&CFG_LOG_CONTROL_FLOW, "log-control-flow", false, "Log control flow (CTS / DTR)")

	helpWanted := pflag.BoolP("help", "h", false, "Show this help dialog")
	pflag.Parse()

	if *helpWanted {
		pflag.Usage()
		os.Exit(0)
	}
}

func main() {
	// Set up logging
	if CFG_OUTPUT_FILE != "" {
		f, err := os.OpenFile(CFG_OUTPUT_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()

		mw := io.MultiWriter(os.Stdout, f)
		log.SetOutput(mw)
	}

	// Parse Options
	serialOptionsLeft, err := ParseSerialOptions(CFG_LEFT_PORT)
	if err != nil {
		log.Fatalf("error opening port to %s: %s\n", CFG_LEFT_PORT_LABEL, err)
	}

	serialOptionsRight, err := ParseSerialOptions(CFG_RIGHT_PORT)
	if err != nil {
		log.Fatalf("error opening port to %s: %s\n", CFG_RIGHT_PORT_LABEL, err)
	}

	// Open Serial Ports
	log.Printf("Opening left serial port (%s): %s", CFG_LEFT_PORT_LABEL, CFG_LEFT_PORT)
	rawPortLeft, err := OpenSerialPort(serialOptionsLeft)
	if err != nil {
		log.Fatalf("Error opening Port to %s: %s\n", CFG_LEFT_PORT_LABEL, err)
	}
	defer rawPortLeft.Close()

	log.Printf("Opening right serial port (%s): %s", CFG_RIGHT_PORT_LABEL, CFG_RIGHT_PORT)
	rawPortRight, err := OpenSerialPort(serialOptionsRight)
	if err != nil {
		log.Fatalf("Error opening Port to %s: %s\n", CFG_RIGHT_PORT_LABEL, err)
	}
	defer rawPortRight.Close()

	// Start the Main Loop
	log.Println("Both ports successfully opened. starting proxy threads... Press return to quit")
	portLeft := SerialPort{
		Port:      rawPortLeft,
		Label:     CFG_LEFT_PORT_LABEL,
		Direction: fmt.Sprintf("%s <- %s", CFG_LEFT_PORT_LABEL, CFG_RIGHT_PORT_LABEL),
	}

	portRight := SerialPort{
		Port:      rawPortRight,
		Label:     CFG_RIGHT_PORT_LABEL,
		Direction: fmt.Sprintf("%s -> %s", CFG_LEFT_PORT_LABEL, CFG_RIGHT_PORT_LABEL),
	}

	ctx, ctxCancel := context.WithCancel(context.TODO())
	var wg sync.WaitGroup

	err = MainLoop(&portLeft, &portRight, ctx, &wg)
	if err != nil {
		log.Println(err)
	}

	ctxCancel()
	wg.Wait()
}
