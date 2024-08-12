// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appartament_building_chat/internal/chat"
	"appartament_building_chat/internal/config"
	"appartament_building_chat/internal/server"
)

func main() {
	cfg, appCfg := config.MustLoad()
	// flag.Parse()
	stop := make(chan error)
	hub := chat.NewHub()
	go hub.Run()
	go server.Start(cfg, hub, appCfg, stop)

	<-stop
}
