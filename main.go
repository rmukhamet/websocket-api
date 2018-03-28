// main
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"gitlab.com/TransportSystem/backend/websocket_service/consul"

	"github.com/micro/go-micro"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/registry"
	bnats "github.com/micro/go-plugins/broker/nats"
	"github.com/nats-io/nats"
)

const (
	natsToken = `x@C7MA*mHLy2#Vcg*R$mPI`
	topic     = "notifications"
)

var (
	hub     = newHub()
	addr    = flag.String("addr", ":7070", "http service address")
	version = "pre-alpha"
)

func sub() {
	_, err := broker.Subscribe(topic, func(p broker.Publication) error {
		hub.inbox <- p.Message()
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to topic %q: %s\n", topic, err)
	}
}

func main() {
	// Работа с подпиской на NATS. Брокер, так брокер...
	broker.DefaultBroker = bnats.NewBroker(
		broker.Addrs(consul.DiscoverNATS()),
		bnats.Options(nats.Options{Token: natsToken}),
	)

	if err := broker.Init(); err != nil {
		log.Fatalf("Broker Init error: %v", err)
	}
	if err := broker.Connect(); err != nil {
		log.Fatalf("Broker Connect error: %v", err)
	}

	sub()

	service := micro.NewService(
		micro.Name("tms-websocket-service"),
		micro.Version(version),
		micro.RegisterTTL(time.Second*30),
		micro.RegisterInterval(time.Second*10),
		micro.Registry(
			registry.NewRegistry(registry.Addrs(consul.ConsulURL)),
		),
	)

	service.Init()

	// Хаб для рассылки сообщений клиентам
	go hub.run()

	// Хэндлер запросов от вебсокет-клиентов
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Сервер для вебсокет-клиентов
	go func() {
		err := http.ListenAndServe(*addr, nil)
		if err != nil {
			log.Fatalln("ListenAndServe: ", err)
		}
	}()

	if err := service.Run(); err != nil {
		log.Println(err.Error())
	}
}
