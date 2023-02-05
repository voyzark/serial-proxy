package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/albenik/go-serial/v2"
)

type SerialData struct {
	CTSset int
	DSRset int
	Data   []byte
}

type SerialPort struct {
	Port      *serial.Port
	Direction string
	Label     string
}

type SerialOptions struct {
	Port     string
	BaudRate int
	Parity   serial.Parity
	DataBits int
	StopBits serial.StopBits
}

func MainLoop(portLeft, portRight *SerialPort, ctx context.Context, wg *sync.WaitGroup) error {
	dataLeftToRight := make(chan SerialData)
	dataRightToLeft := make(chan SerialData)
	errorOccured := make(chan error)

	// Port Left
	go GetControlStatus(portLeft.Port, ctx, wg, dataLeftToRight, errorOccured)
	go ReadBytes(portLeft.Port, ctx, wg, dataLeftToRight, errorOccured)

	// Port Right
	go GetControlStatus(portRight.Port, ctx, wg, dataRightToLeft, errorOccured)
	go ReadBytes(portRight.Port, ctx, wg, dataRightToLeft, errorOccured)

	// User break
	go GetUserInput(errorOccured)

	for {
		select {
		case dataLTR := <-dataLeftToRight:
			go HandleSerialData(portRight, dataLTR, wg, 0, errorOccured)

		case dataRTL := <-dataRightToLeft:
			go HandleSerialData(portLeft, dataRTL, wg, 0, errorOccured)

		case err := <-errorOccured:
			return err
		}
	}
}

func HandleSerialData(port *SerialPort, data SerialData, wg *sync.WaitGroup, delay time.Duration, ec chan error) {
	if data.CTSset != -1 {
		LogControlFlow(port.Direction, "CTS", data.CTSset > 0)
		err := SetRTS(port.Port, data.CTSset > 0)
		if err != nil {
			ec <- fmt.Errorf("error setting RTS on %s port to value %v: %s", port.Label, data.CTSset > 0, err)
			return
		}
	} else if data.DSRset != -1 {
		LogControlFlow(port.Direction, "DSR", data.DSRset > 0)
		err := SetDTR(port.Port, data.DSRset > 0)
		if err != nil {
			ec <- fmt.Errorf("error setting DTR on %s port to value %v: %s", port.Label, data.DSRset > 0, err)
			return
		}
	} else {
		time.Sleep(delay)
		err := WriteBytes(port.Port, data.Data, wg)
		if err != nil {
			ec <- fmt.Errorf("error writing data to %s: %s", port.Label, err)
			return
		}

		LogData(port.Direction, data.Data)
	}
}

func GetControlStatus(ser *serial.Port, ctx context.Context, wg *sync.WaitGroup, c chan SerialData, ec chan error) {
	wg.Add(1)
	defer wg.Done()

	currentCTSStatus := -1
	currentDSRStatus := -1
	for {
		bits, err := ser.GetModemStatusBits()
		if err != nil {
			ec <- fmt.Errorf("error getting status bits: %s", err)
			return
		}

		if bits.CTS && currentCTSStatus < 1 {
			currentCTSStatus = 1
			c <- SerialData{CTSset: 1, DSRset: -1}
		} else if !bits.CTS && currentCTSStatus != 0 {
			currentCTSStatus = 0
			c <- SerialData{CTSset: 0, DSRset: -1}
		}

		if bits.DSR && currentDSRStatus < 1 {
			currentDSRStatus = 1
			c <- SerialData{CTSset: -1, DSRset: 1}
		} else if !bits.DSR && currentDSRStatus != 0 {
			c <- SerialData{CTSset: -1, DSRset: 0}
			currentDSRStatus = 0
		}

		select {
		case <-time.After(10 * time.Millisecond):
			continue
		case <-ctx.Done():
			return
		}
	}
}

func ReadBytes(ser *serial.Port, ctx context.Context, wg *sync.WaitGroup, c chan SerialData, ec chan error) {
	wg.Add(1)
	defer wg.Done()

	buf := make([]byte, CFG_READ_BUFFER_SIZE)
	for {
		n, err := ser.Read(buf)
		if err != nil {
			ec <- fmt.Errorf("error reading serial port: %s", err)
			return
		}

		if n > 0 {
			c <- SerialData{CTSset: -1, DSRset: -1, Data: buf[:n]}
		}

		select {
		case <-time.After(10 * time.Millisecond):
			continue
		case <-ctx.Done():
			return
		}
	}
}

func WriteBytes(ser *serial.Port, data []byte, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	bytesWritten := 0
	for bytesWritten < len(data) {
		n, err := ser.Write(data[bytesWritten:])
		if err != nil {
			return err
		}
		bytesWritten += n
	}

	return nil
}

func SetRTS(ser *serial.Port, value bool) error {
	return ser.SetRTS(value)
}

func SetDTR(ser *serial.Port, value bool) error {
	return ser.SetDTR(value)
}

func GetUserInput(ec chan error) {
	os.Stdin.Read([]byte{0})
	ec <- errors.New("break by user")
}

func LogData(direction string, data []byte) {
	hexDumpLines := strings.Split(hex.Dump(data), "\n")

	// return last newline from hexdump
	if len(hexDumpLines) > 0 {
		hexDumpLines = hexDumpLines[:len(hexDumpLines)-1]
	}

	// add spaces to the beginning of each line, so hexdump aligns with the date & time
	formattedHexDump := "                    " + strings.Join(hexDumpLines, "\n                    ")

	log.Printf("%s\n%s", direction, formattedHexDump)
}

func LogControlFlow(direction, message string, state bool) {
	if !CFG_LOG_CONTROL_FLOW {
		return
	}

	var stateString string
	if state {
		stateString = "set"
	} else {
		stateString = "clear"
	}

	log.Printf("%s: %s %s\n", direction, message, stateString)
}

func OpenSerialPort(options SerialOptions) (*serial.Port, error) {
	port, err := serial.Open(options.Port,
		serial.WithBaudrate(options.BaudRate),
		serial.WithParity(options.Parity),
		serial.WithDataBits(options.DataBits),
		serial.WithStopBits(options.StopBits),
	)
	if err != nil {
		return port, err
	}
	err = port.SetReadTimeout(100)

	return port, err
}

func ParseSerialOptions(optionString string) (SerialOptions, error) {
	var options SerialOptions
	var err error

	var regPattern = regexp.MustCompile(`^(.*?),(\d{3,7}),([neomsNEOMS]),([5678]),(1|1.5|2)$`)
	matches := regPattern.FindStringSubmatch(optionString)
	if matches == nil {
		return options, fmt.Errorf("invalid serial port definition: %s", optionString)
	}

	options.Port = matches[1]
	options.BaudRate, err = strconv.Atoi(matches[2])
	if err != nil {
		return options, fmt.Errorf("invalid serial port definition: %s (inner error: %s)", optionString, err)
	}

	switch strings.ToLower(matches[3]) {
	case "n":
		options.Parity = serial.NoParity
	case "o":
		options.Parity = serial.OddParity
	case "m":
		options.Parity = serial.MarkParity
	case "s":
		options.Parity = serial.SpaceParity
	case "e":
		options.Parity = serial.EvenParity
	default:
		return options, fmt.Errorf("invalid serial port definition: %s", optionString)
	}

	options.DataBits, err = strconv.Atoi(matches[4])
	if err != nil {
		return options, fmt.Errorf("invalid serial port definition: %s (inner error: %s)", optionString, err)
	}

	switch strings.ToLower(matches[5]) {
	case "1":
		options.StopBits = serial.OneStopBit
	case "1.5":
		options.StopBits = serial.OnePointFiveStopBits
	case "2":
		options.StopBits = serial.TwoStopBits
	default:
		return options, fmt.Errorf("invalid serial port definition: %s", optionString)
	}

	return options, nil
}
