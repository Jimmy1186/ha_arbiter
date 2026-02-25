package main

import (
	"kenmec/ha/jimmy/api"
	"kenmec/ha/jimmy/config"
	"kenmec/ha/jimmy/internal"
)

func main() {
	//跟本主機的交管系統連線
	grpcFleetClient := api.NewGRPCFleetClient("localhost:50051")
	go grpcFleetClient.MaintainConnectionWithFleet()
	go grpcFleetClient.StartHeartbeatToFleet()
	go grpcFleetClient.LoggingConnectionStatus()

	// 監聽到另外一台的 HA
	haServer := api.NewHAToOtherServer()
	go haServer.ListenServer(config.Cfg.SERVER_PORT)

	// 連線到另外一台的 HA
	haClient := api.NewGRPCClient(config.Cfg.CLIENT_IP + ":" + config.Cfg.CLIENT_PORT)
	go haClient.MaintainConnection()
	go haClient.LoggingConnectionStatus()

	arbiter := internal.NewArbiter(grpcFleetClient, haClient, haServer)
	go arbiter.MsgHandler()
	go arbiter.StartHeartbeatToOtherHA()
	go arbiter.StartFleetHbMonitor()
	go arbiter.StartOtherHaHbMonitor()

	internal.StartRestWebApi(arbiter)
}
