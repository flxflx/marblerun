// Copyright (c) Edgeless Systems GmbH.
// Licensed under the MIT License.

package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edgelesssys/marblerun/coordinator/config"
	"github.com/edgelesssys/marblerun/coordinator/core"
	"github.com/edgelesssys/marblerun/coordinator/quote"
	"github.com/edgelesssys/marblerun/coordinator/server"
	"github.com/edgelesssys/marblerun/util"
	"go.uber.org/zap"
)

func run(validator quote.Validator, issuer quote.Issuer, sealKey []byte, sealDirPrefix string) {
	// Setup logging with Zap Logger
	var zapLogger *zap.Logger
	var err error

	// Development Logger shows a stacktrace for warnings & errors, Production Logger only for errors
	devMode := os.Getenv(config.DevMode)
	if devMode == "1" {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}

	if err != nil {
		log.Fatal(err)
	}
	defer zapLogger.Sync() // flushes buffer, if any

	zapLogger.Info("starting coordinator")

	// fetching env vars
	sealDir := util.MustGetenv(config.SealDir)
	sealDir = filepath.Join(sealDirPrefix, sealDir)
	dnsNamesString := util.MustGetenv(config.DNSNames)
	dnsNames := strings.Split(dnsNamesString, ",")
	clientServerAddr := util.MustGetenv(config.ClientAddr)
	meshServerAddr := util.MustGetenv(config.MeshAddr)
	etcdNodeName := util.MustGetenv(config.EtcdNodeName)
	etcNamespace := util.MustGetenv(config.EtcdNamespace)
	etcClusterName := util.MustGetenv(config.EtcdClusterName)
	etcClusterSize, err := strconv.Atoi(util.MustGetenv(config.EtcdClusterSize))
	if err != nil {
		panic(err)
	}

	// creating core
	zapLogger.Info("creating the Core object")
	if err := os.MkdirAll(sealDir, 0700); err != nil {
		zapLogger.Fatal("Cannot create or access sealdir. Please check the permissions for the specified path.", zap.Error(err))
	}
	sealer := core.NewAESGCMSealer(sealDir, sealKey)
	core, err := core.NewCore(dnsNames, validator, issuer, sealer, zapLogger)
	if err != nil {
		panic(err)
	}

	// start etcd server
	zapLogger.Info("starting the etcd server")
	go server.RunEtcdServer(etcdNodeName, etcNamespace, etcClusterName, etcClusterSize, devMode == "1", zapLogger)

	// start client server
	zapLogger.Info("starting the client server")
	mux := server.CreateServeMux(core)
	clientServerTLSConfig, err := core.GetTLSConfig()
	if err != nil {
		panic(err)
	}
	go server.RunClientServer(mux, clientServerAddr, clientServerTLSConfig, zapLogger)

	// run marble server
	zapLogger.Info("starting the marble server")
	addrChan := make(chan string)
	errChan := make(chan error)
	go server.RunMarbleServer(core, meshServerAddr, addrChan, errChan, zapLogger)
	for {
		select {
		case err := <-errChan:
			if err != nil {
				panic(err)
			}
			return
		case grpcAddr := <-addrChan:
			zapLogger.Info("started gRPC server", zap.String("grpcAddr", grpcAddr))
		}
	}
}
