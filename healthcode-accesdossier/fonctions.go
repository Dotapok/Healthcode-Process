package main

import (
	"encoding/json"
	"fmt"
	"time"

	modele "healthcode/accesdossier/modeles"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type DossierAccesContract struct {
	contractapi.Contract
}

// HealthcodeDossierAcces.DemandeAccesDossier - Un médecin initie une demande d'accès à un dossier
func (d *DossierAccesContract) DemandeAccesDossier(
	ctx contractapi.TransactionContextInterface,
	demandeID string,
	medecinUUID string,
	patientUUID string,
	consentementID string,
	PrecisionNiveauRequisJSON string,
	etablissement string,
) (*modele.DemandeAcces, error) {
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	// Une session d'accès technique est éphémère (ici fixée à 4 heures par défaut)
	expiration := now.Add(4 * time.Hour)

	demande := modele.DemandeAcces{
		DemandeID:             demandeID,
		MedecinUUID:           medecinUUID,
		PatientUUID:           patientUUID,
		ConsentementID:        consentementID,
		PrecisionNiveauRequis: PrecisionNiveauRequisJSON,
		Etablissement:         etablissement,
		Statut:                modele.EnAttente,
		DateDemande:           now.Format(time.RFC3339),
		DateExpiration:        expiration.Format(time.RFC3339),
	}

	demandeBytes, err := json.Marshal(demande)
	if err != nil {
		return nil, err
	}

	err = ctx.GetStub().PutState(demandeID, demandeBytes)
	if err != nil {
		return nil, err
	}

	return &demande, nil
}

// HealthcodeDossierAcces.AccordAccesDossier - Évalue et valide l'accès en interrogeant Consentement et AuditLog
func (d *DossierAccesContract) AccordAccesDossier(
	ctx contractapi.TransactionContextInterface,
	demandeID string,
	ipHash string,
	deviceID string,
) (string, error) {
	// 1. Récupération de la demande d'accès
	demandeBytes, err := ctx.GetStub().GetState(demandeID)
	if err != nil || demandeBytes == nil {
		return "REFUSE", fmt.Errorf("demande d'acces introuvable : %s", demandeID)
	}

	var demande modele.DemandeAcces
	_ = json.Unmarshal(demandeBytes, &demande)

	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	// Vérification de non-expiration de la demande
	exp, _ := time.Parse(time.RFC3339, demande.DateExpiration)
	if now.After(exp) {
		demande.Statut = modele.Refuse
		db, _ := json.Marshal(demande)
		_ = ctx.GetStub().PutState(demandeID, db)
		return "EXPIRE", fmt.Errorf("la session de demande a expiree")
	}

	// 2. INTERACTION INTER-CHAINCODE N°1 : Appel de HealthcodeConsentement.VerificationNiveauAcces
	// Arguments attendus par HealthcodeConsentement : consentementID, PrecisionNiveauRequisJSON, ipHash, deviceID
	nomChaincodeConsent := "healthcode-consentement"
	argumentsConsent := [][]byte{
		[]byte("VerificationNiveauAcces"),
		[]byte(demande.ConsentementID),
		[]byte(demande.PrecisionNiveauRequis),
		[]byte(ipHash),
		[]byte(deviceID),
	}

	// Invocation sur le même canal
	reponseConsent := ctx.GetStub().InvokeChaincode(nomChaincodeConsent, argumentsConsent, "")
	if reponseConsent.Status != 200 {
		return "REFUSE", fmt.Errorf("erreur lors de la validation du consentement: %s", reponseConsent.Message)
	}

	// Analyse du verdict renvoyé par HealthcodeConsentement
	verdictConsent := string(reponseConsent.Payload)
	if verdictConsent != `"AUTORISE"` && verdictConsent != `AUTORISE` { // Gestion des guillemets JSON strings
		demande.Statut = modele.Refuse
		db, _ := json.Marshal(demande)
		_ = ctx.GetStub().PutState(demandeID, db)
		return verdictConsent, nil // Retourne EXPIRE, NIVEAU_INSUFFISANT ou REFUSE
	}

	// 3. INTERACTION INTER-CHAINCODE N°2 : Appel direct à HealthcodeAuditLog pour acter l'accès au dossier médical
	nomChaincodeAudit := "healthcode-auditlog"
	txID := ctx.GetStub().GetTxID()
	clientOrg, _ := ctx.GetClientIdentity().GetMSPID()

	argumentsAudit := [][]byte{
		[]byte("EnregistreTracePatient"),
		[]byte(txID),
		[]byte(demande.MedecinUUID),
		[]byte(clientOrg),
		[]byte(demande.PatientUUID),
		[]byte("ACCES_DOSSIER"),
		[]byte(demande.PrecisionNiveauRequis),
		[]byte(demande.ConsentementID),
		[]byte(demande.Etablissement),
		[]byte(ipHash),
		[]byte(deviceID),
	}

	reponseAudit := ctx.GetStub().InvokeChaincode(nomChaincodeAudit, argumentsAudit, "")
	if reponseAudit.Status != 200 {
		// ROLLBACK : Si l'audit échoue, on bloque tout l'accès
		return "ERREUR", fmt.Errorf("securite : impossible d'enregistrer la traçabilité de consultation - %s", reponseAudit.Message)
	}

	// 4. Validation finale de l'accès
	demande.Statut = modele.Accorde
	updatedBytes, _ := json.Marshal(demande)
	err = ctx.GetStub().PutState(demandeID, updatedBytes)

	return "AUTORISE", err
}

// HealthcodeAccesDossier.VerifierPermission - Requête de contrôle rapide pour les APIs de lecture (Gateway)
func (d *DossierAccesContract) VerifierPermission(
	ctx contractapi.TransactionContextInterface,
	demandeID string,
) (bool, error) {
	demandeBytes, err := ctx.GetStub().GetState(demandeID)
	if err != nil || demandeBytes == nil {
		return false, nil
	}

	var demande modele.DemandeAcces
	_ = json.Unmarshal(demandeBytes, &demande)

	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	// Vérifie si la session est toujours active, accordée et non expirée dans le temps
	exp, _ := time.Parse(time.RFC3339, demande.DateExpiration)
	if demande.Statut == modele.Accorde && now.Before(exp) {
		return true, nil
	}

	return false, nil
}
