package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	currentLoop "github.com/ceruleandatahub/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tarm/serial"
	"google.golang.org/grpc"
)

const (
	defaultPort        = "/dev/ttyS9"
	defaultBitrate     = 115200
	defaultTimeout     = 60
	defaultRetry       = 5
	defaultGRPCAddress = "0.0.0.0:50051"
)

var config struct {
	port        string
	bitrate     int
	timeout     int
	retry       int
	grpcAddress string
}

func init() {
	config.port = defaultPort
	config.bitrate = defaultBitrate
	config.timeout = defaultTimeout
	config.retry = defaultRetry
	config.grpcAddress = defaultGRPCAddress

	if v := os.Getenv("SERIAL_PORT"); v != "" {
		config.port = v
	}
	if v, err := strconv.Atoi(os.Getenv("SERIAL_BITRATE")); err == nil {
		config.bitrate = v
	}
	if v, err := strconv.Atoi(os.Getenv("SERIAL_TIMEOUT")); err == nil {
		config.timeout = v
	}
	if v := os.Getenv("GRPC_ADDRESS"); v != "" {
		config.grpcAddress = v
	}

	flag.StringVar(&config.port, "port", config.port, "Serial port where the Arduino is connected")
	flag.IntVar(&config.bitrate, "bitrate", config.bitrate, "Serial bitrate used by the Arduino")
	flag.IntVar(&config.timeout, "timeout", config.timeout, "Timeout in seconds to wait for the serial to become available")
	flag.StringVar(&config.grpcAddress, "grpc-address", config.grpcAddress, "gRCP Address to connect")

	// Comment this to get JSON logging, this is for pretty human-readable logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// ConstructTelemetry constructs Telemetry from unstructured string from serial connection 133|0.65|0.43
func ConstructTelemetry(data string, str string) (*currentLoop.CurrentLoopRequest, error) {
	s := strings.Split(data, str)
	if len(s) < 3 {
		return nil, errors.New("Minimum match not found")
	}
	value, err := strconv.Atoi(s[0])
	if err != nil {
		log.Error().Err(err)
		return nil, errors.New("Couldn't parse value")
	}
	voltage, err := strconv.ParseFloat(s[1], 32)
	if err != nil {
		log.Error().Err(err)
		return nil, errors.New("Couldn't parse voltage")
	}
	current, err := strconv.ParseFloat(s[2], 32)
	if err != nil {
		log.Error().Err(err)
		return nil, errors.New("Couldn't parse current")
	}
	timestamp := time.Now().UTC()
	timetext := timestamp.String()
	mac := strings.Join([]string{timetext, data}, "|")
	hasher := sha1.New()
	toHash := []byte(mac)
	hasher.Write(toHash)
	hash := hasher.Sum(nil)
	checksum := base64.URLEncoding.EncodeToString(hash)

	telemetry := &currentLoop.CurrentLoopRequest{
		Hash:      checksum,
		Timestamp: timetext,
		Value:     int32(value),
		Voltage:   float32(voltage),
		Current:   float32(current)}

	return telemetry, nil
}

func main() {
	flag.Parse()

	log.Info().Msg("Starting Arduino Serial Bridge")

	var serialConnection *serial.Port
	var err error

	grpcConnection, err := grpc.Dial(config.grpcAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msg("Could not create gRPC Connection")
		os.Exit(1)
	}
	log.Info().Str("ADDRESS", config.grpcAddress).Msg("Initializing gRPC connection")
	grpcClient := currentLoop.NewCurrentLoopClient(grpcConnection)

	serialConfig := &serial.Config{Name: config.port, Baud: config.bitrate}

	if config.port != "" && config.bitrate > 0 {
		log.Info().Str("PORT", config.port).Int("BITRATE", config.bitrate).Msg("Establishing Serial connection")

		timer := time.NewTicker(time.Duration(config.timeout) * time.Second)
		defer timer.Stop()

		ticker := time.NewTicker(time.Duration(config.retry) * time.Second)
		defer ticker.Stop()

		connected := make(chan bool, 1)

	serialwait:
		for {
			select {
			case <-connected:
				timer.Stop()
				ticker.Stop()
				break serialwait
			case <-timer.C:
				log.Error().Msg("Serial connection timeout")
			case <-ticker.C:
				serialConnection, err = serial.OpenPort(serialConfig)
				if err != nil {
					log.Error().Err(err).Msg("Could not connect to Arduino Serial")
					continue
				}
				defer serialConnection.Close()

				log.Info().Msg("Connected to Arduino Serial")

				connected <- true
				continue
			}
		}
	}

	go func() {
		scanner := bufio.NewScanner(serialConnection)
		for scanner.Scan() {
			data := scanner.Text()
			telemetry, err := ConstructTelemetry(data, "|")
			if err != nil {
				log.Error().Err(err).Msg("Could not construct telemetry message")
				continue
			}
			log.Debug().Str("DATA", data).Msg("Received data from Arduino")

			response, err := grpcClient.SendTelemetry(context.Background(), telemetry)
			if err != nil {
				log.Error().Err(err).Msg("Failed to send telemetry")
				continue
			}
			if response.Hash != telemetry.Hash {
				log.Warn().Msg("Telemetry sent but hash mismatch")
				continue
			} else {
				log.Debug().Str("HASH", response.Hash).Msg("Successfully sent telemetry message")
			}
		}
		if err := scanner.Err(); err != nil {
			log.Error().Err(err).Msg("Failed to scan Arduino Serial")
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
