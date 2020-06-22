package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tarm/serial"
)

const (
	defaultPort    = "/dev/ttyS4"
	defaultBitrate = 115200
	defaultTimeout = 5
)

var config struct {
	port    string
	bitrate int
	socket  string
	timeout int
}

// Telemetry Object 	for JSON Marshalling
type Telemetry struct {
	Hash    string  `json:"hash"`
	Epoch   int     `json:"epoch"`
	Value   int     `json:"value"`
	Voltage float64 `json:"voltage"`
	Current float64 `json:"current"`
}

func init() {
	config.port = defaultPort
	config.bitrate = defaultBitrate
	config.timeout = defaultTimeout

	if v := os.Getenv("SERIAL_PORT"); v != "" {
		config.port = v
	}
	if v := os.Getenv("SOCKET_PATH"); v != "" {
		config.socket = v
	}
	if v, err := strconv.Atoi(os.Getenv("SERIAL_BITRATE")); err == nil {
		config.bitrate = v
	}
	if v, err := strconv.Atoi(os.Getenv("SOCKET_TIMEOUT")); err == nil {
		config.timeout = v
	}

	flag.StringVar(&config.port, "port", config.port, "Serial port where the Arduino is connected")
	flag.IntVar(&config.bitrate, "bitrate", config.bitrate, "Serial bitrate used by the Arduino")
	flag.StringVar(&config.socket, "socket", config.socket, "Path to unix socket where data is written (write to stdout if empty)")
	flag.IntVar(&config.timeout, "timeout", config.timeout, "Timeout in seconds to wait for the socket to become available")

	// Comment this to get JSON logging, this is for pretty human-readable logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// ConstructTelemetry constructs Telemetry from unstructured string from serial connection
func ConstructTelemetry(data string, str string) (telemetry Telemetry, err error) {
	s := strings.Split(data, str)
	if len(s) < 3 {
		return Telemetry{}, errors.New("Minimum match not found")
	}
	value, err := strconv.Atoi(s[0])
	if err != nil {
		log.Error().Err(err)
		return Telemetry{}, errors.New("Couldn't parse value")
	}
	voltage, err := strconv.ParseFloat(s[1], 64)
	if err != nil {
		log.Error().Err(err)
		return Telemetry{}, errors.New("Couldn't parse voltage")
	}
	current, err := strconv.ParseFloat(s[2], 64)
	if err != nil {
		log.Error().Err(err)
		return Telemetry{}, errors.New("Couldn't parse current")
	}

	t := Telemetry{
		Hash:    "hash",
		Epoch:   12345678,
		Value:   value,
		Voltage: voltage,
		Current: current}

	return t, nil
}

func main() {
	flag.Parse()

	log.Info().Msg("Starting Arduino Serial Bridge")

	var socket net.Conn
	var err error

	if config.socket != "" {
		log.Info().Str("PATH", config.socket).Msg("Establishing socket connection")

		timer := time.NewTicker(time.Duration(config.timeout) * time.Second)
		defer timer.Stop()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		connected := make(chan bool, 1)

	socketwait:
		for {
			select {
			case <-connected:
				timer.Stop()
				ticker.Stop()
				break socketwait
			case <-timer.C:
				log.Error().Msg("Socket connection timeout")
			case <-ticker.C:
				socket, err = net.Dial("unix", config.socket)
				if err != nil {
					continue
				}

				log.Info().Msg("Socket connected")
				defer socket.Close()
				connected <- true
				continue
			}
		}
		// socket, err = net.Dial("unix", config.socket)
		// if err != nil {
		// 	log.Fatal().Err(err).Str("SOCKET", config.socket).Msg("Unable to open unix socket")
		// }
		// defer socket.Close()
	}

	serialConfig := &serial.Config{Name: config.port, Baud: config.bitrate}

	conn, err := serial.OpenPort(serialConfig)
	if err != nil {
		log.Info().Msg("Yes, fatal opening connection")
		log.Fatal().Err(err)
		os.Exit(1)
	}
	defer conn.Close()

	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			t, err := ConstructTelemetry(scanner.Text(), "|")
			if err != nil {
				log.Error().Err(err)
				continue
			}
			log.Print(json.Marshal(t))
		}
		if err := scanner.Err(); err != nil {
			log.Fatal().Err(err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigs:
		log.Print("Received an interrupt, stopping...")
		close(done)
	}
	<-done
}
