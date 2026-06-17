package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	consentementContrat := new(ContratIntelligent)

	cc, err := contractapi.NewChaincode(consentementContrat)

	if err != nil {
		log.Panicf("Erreur lors de la creation du chaincode Healthcode Consentement: %v", err)
	}

	cc.Info.Title = "DigiHEALTH Healthcode Consentement"
	cc.Info.Version = "1.0.0"

	if err := cc.Start(); err != nil {
		log.Panicf("Erreur lors du demarrage du Healthcode Consentement: %v", err)
	}
}
