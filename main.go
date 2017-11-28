package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/OpenPeeDeeP/ChessClock-Daemon/chessclock"
	"github.com/OpenPeeDeeP/ChessClock-Daemon/store"
	"github.com/ianschenck/envflag"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

var version string
var (
	daemonConString string
	maxFiles        int
)

func init() {
	envflag.StringVar(&daemonConString, "CCD_CONNECTION_STRING", "localhost:4242", "Connection string to the daemon")
	envflag.IntVar(&maxFiles, "CCD_MAX_FILES", 5, "Maximum number of log files")
}

func main() {
	envflag.Parse()
	lis, err := net.Listen("tcp", daemonConString)
	if err != nil {
		log.Error().Str("con", daemonConString).Msg("failed to listen")
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	ccd := NewDaemon(store.NewFileStore("OpenPeeDeeP", "ChessClock", maxFiles))
	pb.RegisterChessClockServer(grpcServer, ccd)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	go func() {
		s := <-sigc
		switch s {
		case syscall.SIGTERM:
			grpcServer.Stop()
		default:
			grpcServer.GracefulStop()
		}
	}()
	grpcServer.Serve(lis)
}
