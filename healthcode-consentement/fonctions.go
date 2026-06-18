package main

import (
	"encoding/json"
	"fmt"
	modele "healthcode/consentement/modeles"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type ContratIntelligent struct {
	contractapi.Contract
}

type ReponseVerification string

const (
	Autorise             ReponseVerification = "AUTORISE"
	ConsentementInvalide ReponseVerification = "CONSENTEMENT_INVALIDE"
	Refuse               ReponseVerification = "REFUSE"
	Expire               ReponseVerification = "EXPIRE"
	NiveauInsuffisant    ReponseVerification = "NIVEAU_INSUFFISANT"
	Erreur               ReponseVerification = "ERREUR"
)

// HealthcodeConsentement.CreerConsentement - Initialise un nouvel actif de consentement
func (s *ContratIntelligent) CreerConsentement(
	ctx contractapi.TransactionContextInterface,
	consentID string,
	patientUUID string,
	beneficiaireUUID string,
	bType string,
	bOrg string,
	scopeJSON string,
	cType string,
	dateFinStr string,
	nbAccesMax int,
	motif string,
	signature string,
) error {
	// 1. Validation de l'identité de l'appelant (L'appelant doit être le propriétaire du certificat)
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return fmt.Errorf("impossible de recuperer l'identite du client: %v", err)
	}

	// 2. Traitement et désérialisation du Scope
	var scope modele.PrecisionConsentement
	if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
		return fmt.Errorf("format de scope JSON invalide: %v", err)
	}
	if scope.EstVide() {
		return fmt.Errorf("validation échue: la precision du niveau d'acces ne peut pas etre vide")
	}

	// 3. Validation temporelle
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	var now time.Time
	if err != nil {
		now = time.Now() // Fallback si le timestamp Fabric échoue
	} else {
		now = time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))
	}

	nowStr := now.Format(time.RFC3339)

	var dateFinStrOk string
	if modele.TypeConsentement(cType) == modele.Temporaire {
		df, err := time.Parse(time.RFC3339, dateFinStr)
		if err != nil {
			return fmt.Errorf("format date_fin invalide (attendu: RFC3339/ISO8601): %v", err)
		}
		if df.Before(now) {
			return fmt.Errorf("validation échue: date_fin doit être superieure a l'heure actuelle")
		}
		dateFinStrOk = df.Format(time.RFC3339)
	}

	// 4. Construction de l'Asset
	consent := modele.Consentement{
		ConsentementID:             consentID,
		PatientUUID:                patientUUID,
		PatientCertHash:            clientID, // Identification déterministe via le MSP d'infrastructure
		BeneficiaireType:           modele.TypeBeneficiaire(bType),
		BeneficiaireUUID:           beneficiaireUUID,
		BeneficiaireOrg:            bOrg,
		BeneficiairePrecisionAcces: scope,
		TypeConsentement:           modele.TypeConsentement(cType),
		DateDebut:                  nowStr,
		DateFin:                    dateFinStrOk,
		NbAccesMax:                 nbAccesMax,
		NbAccesUses:                0,
		StatutConsentement:         modele.Actif,
		MotifAccorde:               motif,
		DateCreation:               nowStr,
		SignaturePatient:           signature,
	}

	consentBytes, err := json.Marshal(consent)
	if err != nil {
		return err
	}

	// Enregistrement de l'état sur CouchDB (World State)
	return ctx.GetStub().PutState(consentID, consentBytes)
}

// HealthcodeConsentement.VerificationNiveauAcces - Décision de conformité dynamique d'accès en temps réel
func (s *ContratIntelligent) VerificationNiveauAcces(
	ctx contractapi.TransactionContextInterface,
	consentementID string,
	PrecisionNiveauAccesRequisJSON string,
	ipHash string,
	deviceID string,
) (ReponseVerification, error) {
	consentBytes, err := ctx.GetStub().GetState(consentementID)
	if err != nil || consentBytes == nil {
		return Refuse, fmt.Errorf("consentement introuvable sur le registre")
	}

	var consent modele.Consentement
	if err := json.Unmarshal(consentBytes, &consent); err != nil {
		return Refuse, err
	}

	// 1. Vérification du statut global
	if consent.StatutConsentement != modele.Actif {
		return Refuse, nil
	}

	// 2. Vérification de la validité temporelle
	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	if consent.DateFin != "" {
		df, _ := time.Parse(time.RFC3339, consent.DateFin)
		if now.After(df) {
			consent.StatutConsentement = modele.Expire
			updatedBytes, _ := json.Marshal(consent)
			_ = ctx.GetStub().PutState(consentementID, updatedBytes) // Auto-expiration sur le Ledger
			return Expire, nil
		}
	}

	// 3. Vérification des quotas d'accès (Cas du ponctuel)
	if consent.NbAccesMax > 0 && consent.NbAccesUses >= consent.NbAccesMax {
		consent.StatutConsentement = modele.Expire
		updatedBytes, _ := json.Marshal(consent)
		_ = ctx.GetStub().PutState(consentementID, updatedBytes)
		return Expire, nil
	}

	// 4. Vérification granulaire du Scope réquis
	var scopeRequis modele.PrecisionConsentement
	if err := json.Unmarshal([]byte(PrecisionNiveauAccesRequisJSON), &scopeRequis); err != nil {
		return Refuse, fmt.Errorf("format scope requis invalide")
	}

	if !consent.BeneficiairePrecisionAcces.VerifieSiPrecisionSuffisant(scopeRequis) {
		return NiveauInsuffisant, nil
	}

	// 5. Incrémentation du compteur d'accès en cas de succès de la vérification
	consent.NbAccesUses++
	updatedBytes, _ := json.Marshal(consent)
	err = ctx.GetStub().PutState(consentementID, updatedBytes)
	if err != nil {
		return Erreur, fmt.Errorf("impossible de mettre a jour le compteur d'acces : %v", err)
	}

	// 6. LIAISON INTER-CHAINCODE : Appel de HealthcodeAuditLog.EnregistreTracePatient
	// Nous extrayons le TxID actuel de Fabric pour l'utiliser comme LogID unique et lier l'audit à la transaction.
	txID := ctx.GetStub().GetTxID()

	// Récupération dynamique du nom de l'organisation de l'acteur appelant
	clientOrg, _ := ctx.GetClientIdentity().GetMSPID()

	// Préparation des arguments pour AuditLogCC.Record
	// L'ordre doit correspondre EXACTEMENT aux arguments attendus par la méthode Record de l'AuditContract
	chaincodeArgs := [][]byte{
		[]byte("EnregistreTracePatient"),       // Nom de la fonction
		[]byte(txID),                           // logID (On utilise le TxID de Fabric)
		[]byte(consent.BeneficiaireUUID),       // acteur_uuid
		[]byte(clientOrg),                      // acteur_org (Extrait dynamiquement du certificat)
		[]byte(consent.PatientUUID),            // patient_uuid
		[]byte("CONSULTATION"),                 // type_acces
		[]byte(PrecisionNiveauAccesRequisJSON), // scope_accédé
		[]byte(consentementID),                 // consent_id
		[]byte(consent.BeneficiaireOrg),        // etablissement (ou via les arguments de la requête)
		[]byte(ipHash),                         // ip_hash
		[]byte(deviceID),                       // device_id
	}

	// Nom du chaincode cible à appeler (doit correspondre au nom défini lors du déploiement)
	nomChaincodeAudit := "healthcode-auditlog"

	// Le canal cible. Si vide "", Fabric utilise le canal courant de la transaction actuelle (Recommandé).
	canalCible := ""

	// Exécution de l'appel inter-chaincode
	reponseAudit := ctx.GetStub().InvokeChaincode(nomChaincodeAudit, chaincodeArgs, canalCible)
	if reponseAudit.Status != 200 {
		// SÉCURITÉ CRITIQUE : Si l'enregistrement de l'audit échoue, on DOIT faire échouer toute la transaction.
		// Cela annule (rollback) l'incrémentation du compteur d'accès écrite plus haut. Pas de log = Pas d'accès.
		return Erreur, fmt.Errorf("securite : echec de l'enregistrement immuable de l'audit (Status: %d) - %s",
			reponseAudit.Status, reponseAudit.Message)
	}

	return Autorise, nil
}

// HealthcodeConsentement.RevocationConcentement - Révocation immédiate et unilatérale par le patient
func (s *ContratIntelligent) RevocationConcentement(
	ctx contractapi.TransactionContextInterface,
	consentementID string,
	patientUUID string,
	motif string,
) error {
	consentBytes, err := ctx.GetStub().GetState(consentementID)
	if err != nil || consentBytes == nil {
		return fmt.Errorf("consentement introuvable")
	}

	var consent modele.Consentement
	_ = json.Unmarshal(consentBytes, &consent)

	// Validation de sécurité : Seul le propriétaire légitime peut révoquer son document
	if consent.PatientUUID != patientUUID {
		return fmt.Errorf("violation d'acces : ce consentement n'appartient pas au patient requérant")
	}

	if consent.StatutConsentement != modele.Actif {
		return fmt.Errorf("impossible de révoquer un consentement non actif (Statut Actuel: %s)", consent.StatutConsentement)
	}

	txTimestamp, _ := ctx.GetStub().GetTxTimestamp()
	now := time.Unix(txTimestamp.Seconds, int64(txTimestamp.Nanos))

	consent.StatutConsentement = modele.Revoque
	consent.MotifRefus = motif
	consent.DateRevocation = now.Format(time.RFC3339)

	updatedBytes, _ := json.Marshal(consent)
	return ctx.GetStub().PutState(consentementID, updatedBytes)
}

// HealthcodeConsentement.SuspensionConsentement - Commande administrative / injonction juridique
func (s *ContratIntelligent) SuspensionConsentement(
	ctx contractapi.TransactionContextInterface,
	consentementID string,
	motifJuridique string,
) error {
	// Vérification du rôle via l'Attribut basés sur l'Identité (ABAC)
	// L'identité de l'appelant doit posséder explicitement l'attribut "hf.Affiliation" == "DigiHEALTH.admin"
	err := ctx.GetClientIdentity().AssertAttributeValue("role", "admin")
	if err != nil {
		return fmt.Errorf("acces refuse : privilege administrateur DigiHEALTH requis")
	}

	consentBytes, err := ctx.GetStub().GetState(consentementID)
	if err != nil || consentBytes == nil {
		return fmt.Errorf("consentement introuvable")
	}

	var consent modele.Consentement
	_ = json.Unmarshal(consentBytes, &consent)

	consent.StatutConsentement = modele.Suspendu
	consent.MotifRefus = "SUSPENSION: " + motifJuridique

	updatedBytes, _ := json.Marshal(consent)
	return ctx.GetStub().PutState(consentementID, updatedBytes)
}

// HealthcodeConsentement.RecupererConsentementsPatient - Liste les consentements en utilisant des index CouchDB optimisés
func (s *ContratIntelligent) RecupererConsentementsPatient(
	ctx contractapi.TransactionContextInterface,
	patientUUID string,
) ([]*modele.Consentement, error) {
	// Utilisation d'un sélecteur Rich Query CouchDB optimisé pour éviter les scans complets (Full Table Scan)
	queryString := fmt.Sprintf(`{"selector":{"patient_uuid":"%s"}}`, patientUUID)

	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var consents []*modele.Consentement
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var consent modele.Consentement
		if err := json.Unmarshal(queryResponse.Value, &consent); err != nil {
			return nil, err
		}
		consents = append(consents, &consent)
	}

	return consents, nil
}

// HealthcodeConsentement.RecupererHistoriqueConsentementPrecisParID - Récupère l'historique complet immuable (provenance) depuis la base de données d'historique Fabric
func (s *ContratIntelligent) RecupererHistoriqueConsentementPrecisParID(
	ctx contractapi.TransactionContextInterface,
	consentementID string,
) ([]map[string]interface{}, error) {
	resultsIterator, err := ctx.GetStub().GetHistoryForKey(consentementID)
	if err != nil {
		return nil, fmt.Errorf("impossible de lire l'historique de la clef: %v", err)
	}
	defer resultsIterator.Close()

	var historyList []map[string]interface{}
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		txInfo := make(map[string]interface{})
		txInfo["tx_id"] = response.TxId
		txInfo["is_delete"] = response.IsDelete

		var consent modele.Consentement
		if len(response.Value) > 0 {
			_ = json.Unmarshal(response.Value, &consent)
			txInfo["value"] = consent
		}

		txInfo["timestamp"] = time.Unix(response.Timestamp.Seconds, int64(response.Timestamp.Nanos)).Format(time.RFC3339)
		historyList = append(historyList, txInfo)
	}

	return historyList, nil
}
