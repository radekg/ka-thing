package start

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/radekg/ka-thing/config"
	"github.com/radekg/ka-thing/do"
	"github.com/spf13/cobra"
)

// Command is the build command declaration.
var Command = &cobra.Command{
	Use:   "start",
	Short: "Runs the proxy",
	Run:   run,
	Long:  ``,
}

var (
	proxyConfig = config.NewProxyConfig()
	logConfig   = config.NewLoggingConfig()
)

func initFlags() {
	Command.Flags().AddFlagSet(proxyConfig.FlagSet())
	Command.Flags().AddFlagSet(logConfig.FlagSet())

}

func init() {
	initFlags()
}

func run(cobraCommand *cobra.Command, args []string) {
	os.Exit(processCommand(args))
}

func processCommand(args []string) int {

	logger := logConfig.NewLogger("proxy")

	proxyContext, cancelFunc := context.WithCancel(context.Background())

	s := do.NewProxy(proxyContext, proxyConfig, logger)
	go s.Start()

	select {
	case <-s.ReadyNotify():
		// server is running...
		logger.Info("server running")
	case <-s.FailedNotify():
		logger.Error("startup failed", "reason", s.StartFailureReason())
		cancelFunc()
		return 1
	case <-time.After(time.Second * 10):
		logger.Error("startup failed, timed out")
		cancelFunc()
		return 2
	}

	waitForStop()

	cancelFunc()

	<-s.StoppedNotify()

	return 0
}

func waitForStop() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
