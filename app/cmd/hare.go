package cmd

import (
	"fmt"
	cfg "github.com/spacemeshos/go-spacemesh/config"
	"github.com/spacemeshos/go-spacemesh/hare"
	hc "github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/oracle"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spf13/cobra"
	"time"
)

var value1 = hare.Value{Bytes32: hare.Bytes32{1}}

// VersionCmd returns the current version of spacemesh
var HareCmd = &cobra.Command{
	Use:   "hare",
	Short: "start hare",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting hare")
		hareApp := NewHareApp()
		defer hareApp.Cleanup()
		hareApp.Start(cmd, args)
		<-hareApp.proc.CloseChannel()
	},
}

func init() {
	RootCmd.AddCommand(HareCmd)
}

type HareApp struct {
	p2p    p2p.Service
	Config *cfg.Config
	broker *hare.Broker
	proc   *hare.ConsensusProcess
	oracle *oracle.OracleClient
	sgn    hare.Signing
}

func NewHareApp() *HareApp {
	dc := cfg.DefaultConfig()
	return &HareApp{sgn:hare.NewMockSigning(), Config: &dc}
}

func (app *HareApp) Cleanup() {
	app.oracle.Unregister(true, app.sgn.Verifier().String())
}

func (app *HareApp) Start(cmd *cobra.Command, args []string) {
	// start p2p services
	log.Info("Initializing P2P services")
	swarm, err := p2p.New(Ctx, app.Config.P2P)
	app.p2p = swarm
	if err != nil {
		log.Error("Error starting p2p services, err: %v", err)
		panic("Error starting p2p services")
	}

	pub := app.sgn.Verifier()

	lg := log.NewDefault(pub.String())

	oracle.SetServerAddress(app.Config.OracleServer)
	app.oracle = oracle.NewOracleClientWithWorldID(app.Config.OracleServerWorldId)
	app.oracle.Register(true, pub.String()) // todo: configure no faulty nodes
	hareOracle := oracle.NewHareOracleFromClient(app.oracle)

	cfg := hc.Config{N: 1, F: 0, RoundDuration: 3000 * time.Millisecond}
	broker := hare.NewBroker(swarm, hare.NewEligibilityValidator(hare.NewHareOracle(hareOracle, cfg.N), lg))
	app.broker = broker
	broker.Start()
	proc := hare.NewConsensusProcess(cfg, 1, hare.NewSetFromValues(value1), hareOracle, app.sgn, swarm, make(chan hare.TerminationOutput, 1), lg)
	app.proc = proc
	proc.SetInbox(broker.Register(proc.Id()))
	proc.Start()
}
