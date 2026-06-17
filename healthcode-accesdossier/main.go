package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	dossierContract := new(DossierAccesContract)

	cc, err := contractapi.NewChaincode(dossierContract)
	if err != nil {
		log.Panicf("Erreur lors de la creation du chaincode Healthcode AccesDossier: %v", err)
	}

	cc.Info.Title = "DigiHEALTH Healthcode AccesDossier"
	cc.Info.Version = "1.0.0"

	if err := cc.Start(); err != nil {
		log.Panicf("Erreur lors du demarrage du chaincode Healthcode AccesDossier: %v", err)
	}
}
