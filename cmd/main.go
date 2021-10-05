package main

import (
	"net/http"
	"os"

	proxyDelivery "github.com/Snikimonkd/mitm_proxy/internal/proxy/delivery"
	proxyRepos "github.com/Snikimonkd/mitm_proxy/internal/proxy/repository"
	proxyUcase "github.com/Snikimonkd/mitm_proxy/internal/proxy/usecase"
	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

func main() {
	conn, err := sqlx.Connect("pgx",
		"postgres://"+os.Getenv("DB_USER")+":proxyuser@localhost:5432/"+os.Getenv("DB_NAME"))
	if err != nil {
		logrus.Info(err)
	}

	conn.SetMaxOpenConns(8)
	conn.SetMaxIdleConns(8)

	if err := conn.Ping(); err != nil {
		logrus.Info(err)
	}

	defer conn.Close()

	proxyR := proxyRepos.NewProxyRepos(conn)
	proxyU := proxyUcase.NewProxyUcase(proxyR)

	proxyD := proxyDelivery.NewProxyDelivery(proxyU)

	server := &http.Server{
		Addr:    ":" + os.Getenv("PROXY_PORT"),
		Handler: http.HandlerFunc(proxyD.HandleRequest),
	}

	go func() {
		logrus.Info(server.ListenAndServe())
	}()

	router := mux.NewRouter()
	router.HandleFunc("/requests", proxyD.GetAllRequestsHandler)
	router.HandleFunc("/requests/{id:[0-9]+}", proxyD.GetRequestHandler)
	router.HandleFunc("/repeat/{id:[0-9]+}", proxyD.RepeatRequestHandler)
	router.HandleFunc("/scan/{id:[0-9]+}", proxyD.ScanRequestHandler)
	http.Handle("/", router)
	logrus.Info(http.ListenAndServe(":"+os.Getenv("REPEATER_PORT"), nil))
}
