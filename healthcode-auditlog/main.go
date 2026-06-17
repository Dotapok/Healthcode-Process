package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	contratAudit := new(ContratAudit)

	cc, err := contractapi.NewChaincode(contratAudit)
	if err != nil {
		log.Panicf("Erreur lors de la création du chaincode Healthcode AuditLog: %v", err)
	}

	cc.Info.Title = "DigiHEALTH Healthcode AuditLog"
	cc.Info.Version = "1.0.0"

	if err := cc.Start(); err != nil {
		log.Panicf("Erreur lors du démarrage du chaincode Healthcode AuditLog: %v", err)
	}
}
