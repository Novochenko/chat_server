package server

import (
	chat "appartament_building_chat/internal/chat"
	ssogrpc "appartament_building_chat/internal/clients/sso/grpc"
	"appartament_building_chat/internal/config"
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
)

func Start(cfg *config.Config, hub *chat.Hub, appCfg *config.AppConfig, stop chan error) {
	logS := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ssoClient, err := ssogrpc.New(
		context.Background(),
		logS,
		cfg.Clients.SSO.Adress,
		cfg.Clients.SSO.Timeout,
		cfg.Clients.SSO.RetriesCount,
	)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	server := newServer(cfg, appCfg, hub, ssoClient)
	err = http.ListenAndServe(cfg.BindAddr, server)
	if err != nil {
		stop <- err
	}

}
