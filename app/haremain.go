package app

//import (
//	"fmt"
//	"github.com/spacemeshos/go-spacemesh/hare"
//	hc "github.com/spacemeshos/go-spacemesh/hare/config"
//	"github.com/spacemeshos/go-spacemesh/log"
//	"github.com/spacemeshos/go-spacemesh/oracle"
//	"github.com/spacemeshos/go-spacemesh/p2p"
//	"github.com/spf13/cobra"
//	"os"
//	"time"
//)
//
//var value1 = hare.Value{Bytes32: hare.Bytes32{1}}
//
//
//type HareApp struct {
//	*SpacemeshApp
//	broker *hare.Broker
//	proc   *hare.ConsensusProcess
//}
//
//func NewHareApp() *HareApp {
//	app := NewSpacemeshApp()
//	hApp := &HareApp{SpacemeshApp:app}
//	app.Command.Run = hApp.startHare
//
//	return hApp
//}
//
//func (app *HareApp) startHare(cmd *cobra.Command, args []string) {
//	// start p2p services
//	log.Info("Initializing P2P services")
//	swarm, err := p2p.New(Ctx, app.Config.P2P)
//	if err != nil {
//		log.Error("Error starting p2p services, err: %v", err)
//		panic("Error starting p2p services")
//	}
//
//	sgn := hare.NewMockSigning()
//	pub := sgn.Verifier()
//
//	lg := log.NewDefault(pub.String())
//
//	oracle.SetServerAddress(app.Config.OracleServer)
//	oracleClient := oracle.NewOracleClientWithWorldID(app.Config.OracleServerWorldId)
//	oracleClient.Register(true, pub.String()) // todo: configure no faulty nodes
//	hareOracle := oracle.NewHareOracleFromClient(oracleClient)
//
//	cfg := hc.DefaultConfig()
//	broker := hare.NewBroker(swarm, hare.NewEligibilityValidator(hare.NewHareOracle(hareOracle, cfg.N), lg))
//	proc := hare.NewConsensusProcess(cfg, 1, hare.NewSetFromValues(value1), hareOracle, sgn, swarm, make(chan hare.TerminationOutput), lg)
//	broker.Register(proc)
//	proc.Start()
//	broker.Start()
//}
//
//// Main is the entry point for the Spacemesh console app - responsible for parsing and routing cli flags and commands.
//// This is the root of all evil, called from Main.main().
//func MainHare() {
//	log.Info("Starting hare app instance")
//
//	myApp := NewHareApp()
//
//	if err := myApp.Execute(); err != nil {
//		fmt.Fprintln(os.Stderr, err)
//		os.Exit(1)
//	}
//
//	EntryPointCreated <- true
//
//	time.Sleep(10*time.Second)
//}