/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package packetinjector

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

const (
	// Namespace Packet_Injector
	Namespace = "Packet_Injector"
)

// Server creates a packet injector server API
type Server struct {
	Graph    *graph.Graph
	Channels *Channels
}

func (pis *Server) stopPI(msg *ws.StructMessage) error {
	var uuid string
	if err := json.Unmarshal(msg.Obj, &uuid); err != nil {
		return err
	}
	pis.Channels.Lock()
	c, ok := pis.Channels.Pipes[uuid]
	pis.Channels.Unlock()
	if ok {
		c <- true
		return nil
	}
	return fmt.Errorf("No PI running on this ID: %s", uuid)
}

func (pis *Server) injectPacket(msg *ws.StructMessage) (string, error) {
	var params PacketInjectionParams
	if err := json.Unmarshal(msg.Obj, &params); err != nil {
		return "", fmt.Errorf("Unable to decode packet inject param message %v", msg)
	}

	trackingID, err := InjectPackets(&params, pis.Graph, pis.Channels)
	if err != nil {
		return "", fmt.Errorf("Failed to inject packet: %s", err.Error())
	}

	return trackingID, nil
}

// OnStructMessage event, websocket PIRequest message
func (pis *Server) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	switch msg.Type {
	case "PIRequest":
		var reply *ws.StructMessage
		trackingID, err := pis.injectPacket(msg)
		replyObj := &Reply{TrackingID: trackingID}
		if err != nil {
			logging.GetLogger().Error(err)

			replyObj.Error = err.Error()
			reply = msg.Reply(replyObj, "PIResult", http.StatusBadRequest)
		} else {
			reply = msg.Reply(replyObj, "PIResult", http.StatusOK)
		}

		c.SendMessage(reply)
	case "PIStopRequest":
		var reply *ws.StructMessage
		err := pis.stopPI(msg)
		replyObj := &Reply{}
		if err != nil {
			replyObj.Error = err.Error()
			reply = msg.Reply(replyObj, "PIStopResult", http.StatusBadRequest)
		} else {
			reply = msg.Reply(replyObj, "PIStopResult", http.StatusOK)
		}
		c.SendMessage(reply)
	}
}

// NewServer creates a new packet injector server API based on websocket server
func NewServer(graph *graph.Graph, pool ws.StructSpeakerPool) *Server {
	s := &Server{
		Graph:    graph,
		Channels: &Channels{Pipes: make(map[string](chan bool))},
	}
	pool.AddStructMessageHandler(s, []string{Namespace})
	return s
}
