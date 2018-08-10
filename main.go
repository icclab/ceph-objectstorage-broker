package main

import (
	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
	"github.engineering.zhaw.ch/kaio/ceph-objectstore-broker/broker"
	"github.engineering.zhaw.ch/kaio/ceph-objectstore-broker/brokerConfig"
	rg "github.engineering.zhaw.ch/kaio/ceph-objectstore-broker/radosgw"
	"github.engineering.zhaw.ch/kaio/ceph-objectstore-broker/s3"
	"github.engineering.zhaw.ch/kaio/ceph-objectstore-broker/utils"
	"net/http"
	"os"
)

func main() {
	//Init logger
	logger := lager.NewLogger("broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))
	logger.Debug("Starting")

	//Load configs
	bc := &brokerConfig.BrokerConfig{}
	err := bc.Update()
	if err != nil {
		logger.Error("Failed to load broker config", err)
		return
	}

	services := []brokerapi.Service{}
	err = utils.LoadJsonFromFile("brokerConfig/service-config.json", &services)
	if err != nil {
		logger.Error("Failed to load service config", err)
		return
	}

	//Connect to rgw
	rados := &rg.Radosgw{}
	if err := rados.Setup(bc.RadosEndpoint, bc.RadosAdminPath, bc.RadosAccessKey, bc.RadosSecretKey); err != nil {
		logger.Error("Failed to connect to radosgw", err)
		return
	}

	//Create s3 client
	s := &s3.S3{}
	err = s.Connect(bc.RadosEndpoint, bc.RadosAccessKey, bc.RadosSecretKey, false)
	if err != nil {
		logger.Error("Failed to connect to S3", err)
		return
	}

	brok := &broker.Broker{
		Logger:            logger,
		Rados:             rados,
		ServiceConfig:     services,
		BrokerConfig:      bc,
		S3:                s,
		ShouldReturnAsync: false,
	}

	if b, _ := s.BucketExists(broker.BucketName); !b {
		if err = s.CreateBucket(broker.BucketName); err != nil {
			logger.Error("Failed to create base bucket for the broker", err)
			return
		}
	}

	//Start the broker
	creds := brokerapi.BrokerCredentials{Username: bc.BrokerUsername, Password: bc.BrokerPassword}
	handler := brokerapi.New(brok, logger, creds)
	http.Handle("/", handler)
	logger.Debug("Listen and serve on port: 8080")
	_ = http.ListenAndServe(":8080", nil)
}
