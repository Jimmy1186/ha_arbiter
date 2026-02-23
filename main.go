package main

import (
	"kenmec/ha/jimmy/api"
	"kenmec/ha/jimmy/config"
	"log"
	"time"
)

func main() {
	//跟本主機的交管系統連線
	grpcFleetClient := api.NewGRPCFleetClient("localhost:50051")
	go grpcFleetClient.MaintainConnectionWithFleet()
	go grpcFleetClient.StartHeartbeatToFleet(30 * time.Second)

	for !grpcFleetClient.IsConnectedToFleet() {
		log.Println("⏳ 等待 gRPC 連線到交管...")
		time.Sleep(1 * time.Second)
	}
	// 監聽到另外一台的 HA
	haServer := api.NewHAToOtherServer()
	go haServer.ListenServer(config.Cfg.SERVER_PORT)

	// 連線到另外一台的 HA
	haClient := api.NewGRPCClient(config.Cfg.CLIENT_IP + ":" + config.Cfg.CLIENT_PORT)
	go haClient.MaintainConnection()

	for !haClient.IsConnected() {
		log.Println("⏳ 等待 HA gRPC 連線到另外一台HA...")
		time.Sleep(1 * time.Second)
	}

	select {}
}
