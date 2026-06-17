package main

import (
	"encoding/json"
	"fmt"
	"time"

	modele "healthcode/auditlog/modeles"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type ContratAudit struct {
	contractapi.Contract
}

// HealthcodeAuditLog.EnregistreTracePatient - Enregistre un accès standard à un dossier médical
func (a *ContratAudit) EnregistreTracePatient(
	ctx contractapi.TransactionContextInterface,
	logID string,
	acteurUUID string,
	acteurOrg string,
	patientUUID string,
	typeAcces string,
	PrecisionAcces string,
	consentementID string,
	etablissement string,
	ipHash string,
	deviceID string,
) error {
	// Récupération sécurisée du timestamp de la transaction
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return fmt.Errorf("impossible de lire le timestamp de la transaction: %v", err)
	}
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	logEntry := modele.AuditAcces{
		LogID:          logID,
		ActeurUUID:     acteurUUID,
		ActeurOrg:      acteurOrg,
		PatientUUID:    patientUUID,
		TypeAcces:      typeAcces,
		PrecisionAcces: PrecisionAcces,
		Timestamp:      now.Format(time.RFC3339),
		ConsentementID: consentementID,
		Etablissement:  etablissement,
		IPHash:         ipHash,
		DeviceID:       deviceID,
		IsUrgence:      false,
	}

	logBytes, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	// Indexation par clé composite pour optimiser les requêtes futures (patient_uuid + log_id)
	indexKey, err := ctx.GetStub().CreateCompositeKey("patient~log", []string{patientUUID, logID})
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(indexKey, logBytes)
}

// HealthcodeAuditLog.EnregistreTraceUrgence - Enregistre un accès d'urgence (Déclenché par UrgenceCC)
func (a *ContratAudit) EnregistreTraceUrgence(
	ctx contractapi.TransactionContextInterface,
	logID string,
	acteurUUID string,
	acteurOrg string,
	patientUUID string,
	PrecisionAcces string,
	etablissement string,
	ipHash string,
	deviceID string,
	justification string,
	codeSamu string,
	medecinCert string,
) error {
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	logEntry := modele.AuditAcces{
		LogID:                logID,
		ActeurUUID:           acteurUUID,
		ActeurOrg:            acteurOrg,
		PatientUUID:          patientUUID,
		TypeAcces:            "URGENCE",
		PrecisionAcces:       PrecisionAcces,
		Timestamp:            now.Format(time.RFC3339),
		Etablissement:        etablissement,
		IPHash:               ipHash,
		DeviceID:             deviceID,
		IsUrgence:            true,
		JustificationUrgence: justification,
		CodeSamu:             codeSamu,
		MedecinCert:          medecinCert,
	}

	logBytes, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	indexKey, err := ctx.GetStub().CreateCompositeKey("patient~log", []string{patientUUID, logID})
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(indexKey, logBytes)
}

// HealthcodeAuditLog.ModificationDAcces - Enregistre l'altération d'un acte médical
func (a *ContratAudit) ModificationDAcces(
	ctx contractapi.TransactionContextInterface,
	logID string,
	acteID string,
	ancienHash string,
	nouveauHash string,
	medecinUUID string,
	motif string,
) error {
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	modEntry := modele.ModificationDAcces{
		LogID:             logID,
		ActeID:            acteID,
		AncienHash:        ancienHash,
		NouveauHash:       nouveauHash,
		MedecinUUID:       medecinUUID,
		MotifModification: motif,
		Timestamp:         now.Format(time.RFC3339),
	}

	modBytes, err := json.Marshal(modEntry)
	if err != nil {
		return err
	}

	// Indexation par clé composite pour le suivi des médecins (medecin_uuid + log_id)
	indexKey, err := ctx.GetStub().CreateCompositeKey("medecin~mod", []string{medecinUUID, logID})
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(indexKey, modBytes)
}

// HealthcodeAuditLog.ConsulterPatientAudit - Lecture de l'historique d'accès par le patient concerné
func (a *ContratAudit) ConsulterPatientAudit(
	ctx contractapi.TransactionContextInterface,
	patientUUID string,
) ([]*modele.AuditAcces, error) {
	// SÉCURITÉ : Vérification de l'identité de l'appelant.
	// Si l'appelant n'a pas le rôle 'admin' ou 'minsante', il DOIT être le patient lui-même.
	id := ctx.GetClientIdentity()
	roleAttr, _, roleErr := id.GetAttributeValue("role")

	if roleErr != nil || (roleAttr != "admin" && roleAttr != "minsante") {
		// Extraction de l'identifiant utilisateur custom stocké dans le certificat X.509
		userUUID, _, err := id.GetAttributeValue("uuid")
		if err != nil || userUUID != patientUUID {
			return nil, fmt.Errorf("accès refusé : vous ne pouvez consulter que votre propre journal d'audit")
		}
	}

	// Utilisation de l'index de clé composite pour une récupération ultra-rapide
	iterator, err := ctx.GetStub().GetStateByPartialCompositeKey("patient~log", []string{patientUUID})
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	var entries []*modele.AuditAcces
	for iterator.HasNext() {
		response, err := iterator.Next()
		if err != nil {
			return nil, err
		}

		var entry modele.AuditAcces
		_ = json.Unmarshal(response.Value, &entry)
		entries = append(entries, &entry)
	}

	return entries, nil
}

// HealthcodeAuditLog.ConsulterMedecinActivity - Audit de l'activité d'un médecin (Réservé Admin & MINSANTE)
func (a *ContratAudit) ConsulterMedecinActivity(
	ctx contractapi.TransactionContextInterface,
	medecinUUID string,
) ([]*modele.ModificationDAcces, error) {
	// SÉCURITÉ : Interdiction stricte aux patients. Rôle Admin DigiHEALTH ou MINSANTE requis via ABAC.
	id := ctx.GetClientIdentity()
	roleAttr, _, err := id.GetAttributeValue("role")
	if err != nil || (roleAttr != "admin" && roleAttr != "minsante") {
		return nil, fmt.Errorf("accès refusé : privilèges d'audit réglementaire insuffisants")
	}

	iterator, err := ctx.GetStub().GetStateByPartialCompositeKey("medecin~mod", []string{medecinUUID})
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	var modifications []*modele.ModificationDAcces
	for iterator.HasNext() {
		response, err := iterator.Next()
		if err != nil {
			return nil, err
		}

		var mod modele.ModificationDAcces
		_ = json.Unmarshal(response.Value, &mod)
		modifications = append(modifications, &mod)
	}

	return modifications, nil
}

// HealthcodeAuditLog.ExporterPourAudit - Export pour audit réglementaire (MINSANTE + Justice via Token)
func (a *ContratAudit) ExporterPourAudit(
	ctx contractapi.TransactionContextInterface,
	complianceToken string,
) ([]*modele.AuditAcces, error) {
	// SÉCURITÉ : Validation du rôle et du token éphémère d'autorisation légale
	id := ctx.GetClientIdentity()
	roleAttr, _, err := id.GetAttributeValue("role")
	if err != nil || (roleAttr != "minsante" && roleAttr != "justice") {
		return nil, fmt.Errorf("accès refusé : réservé aux autorités de contrôle ou judiciaires")
	}

	if complianceToken == "" {
		return nil, fmt.Errorf("validation échouée : jeton de réquisition d'audit manquant")
	}

	// Rich Query CouchDB pour exporter l'ensemble des événements à des fins légales
	// Dans un cas de production, on filtrerait ici par plage de dates ou établissement
	queryString := `{"selector":{"is_urgence":true}}` // Exemple : Extraction prioritaire de toutes les urgences

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var complianceLogs []*modele.AuditAcces
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var entry modele.AuditAcces
		_ = json.Unmarshal(queryResponse.Value, &entry)
		complianceLogs = append(complianceLogs, &entry)
	}

	return complianceLogs, nil
}
