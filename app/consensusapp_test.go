package app

import (
	"fmt"
	"github.com/spacemeshos/go-spacemesh/hare"
	"github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/oracle"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spf13/cobra"
	"os"
	"testing"
	"time"
)

var value1 = hare.Value{Bytes32: hare.Bytes32{1}}

type HareApp struct {
	*SpacemeshApp
	broker *hare.Broker
	proc   *hare.ConsensusProcess
}

func newHareApp() *HareApp {
	app := newSpacemeshApp()
	hApp := &HareApp{SpacemeshApp:app}
	app.Command.Run = hApp.startHare

	return hApp
}

func (app *HareApp) startHare(cmd *cobra.Command, args []string) {
	// start p2p services
	log.Info("Initializing P2P services")
	swarm, err := p2p.New(Ctx, app.Config.P2P)
	if err != nil {
		log.Error("Error starting p2p services, err: %v", err)
		panic("Error starting p2p services")
	}

	sgn := hare.NewMockSigning()
	pub := sgn.Verifier()

	lg := log.NewDefault(pub.String())

	oracle.SetServerAddress(app.Config.OracleServer)
	oracleClient := oracle.NewOracleClientWithWorldID(app.Config.OracleServerWorldId)
	oracleClient.Register(true, pub.String()) // todo: configure no faulty nodes
	hareOracle := oracle.NewHareOracleFromClient(oracleClient)

	cfg := config.DefaultConfig()
	broker := hare.NewBroker(swarm, hare.NewEligibilityValidator(hare.NewHareOracle(hareOracle, cfg.N), lg))
	proc := hare.NewConsensusProcess(cfg, 1, hare.NewSetFromValues(value1), hareOracle, sgn, swarm, make(chan hare.TerminationOutput), lg)
	broker.Register(proc)
	proc.Start()
	broker.Start()
}

func Test_ConsensusApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	app := newHareApp()

	if err := app.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	EntryPointCreated <- true

	time.Sleep(10*time.Second)
}
