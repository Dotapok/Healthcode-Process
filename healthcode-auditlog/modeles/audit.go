package modeles

// AuditAcces représente un accès standard ou d'urgence à un dossier médical
type AuditAcces struct {
	LogID          string `json:"log_id"` // UUID v7 ou TxID Fabric
	ActeurUUID     string `json:"acteur_uuid"`
	ActeurOrg      string `json:"acteur_org"`
	PatientUUID    string `json:"patient_uuid"`
	TypeAcces      string `json:"type_acces"`
	PrecisionAcces string `json:"precision_acces"`
	Timestamp      string `json:"timestamp"`
	ConsentementID string `json:"consentement_id,omitempty"`
	Etablissement  string `json:"etablissement"`
	IPHash         string `json:"ip_hash"`
	DeviceID       string `json:"device_id"`

	// Champs spécifiques aux accès d'urgence
	IsUrgence            bool   `json:"is_urgence"`
	JustificationUrgence string `json:"justification_urgence,omitempty"`
	CodeSamu             string `json:"code_samu,omitempty"`
	MedecinCert          string `json:"medecin_cert,omitempty"`
}

// ModificationDacces représente l'historique d'altération/mise à jour d'un acte médical
type ModificationDAcces struct {
	LogID             string `json:"log_id"`
	ActeID            string `json:"acte_id"`
	AncienHash        string `json:"ancien_hash"`
	NouveauHash       string `json:"nouveau_hash"`
	MedecinUUID       string `json:"medecin_uuid"`
	MotifModification string `json:"motif_modification"`
	Timestamp         string `json:"timestamp"`
}

// CreationDocument représente l'ancrage initial de l'empreinte d'un document médical (ordonnance, compte rendu, etc.)
type CreationDocument struct {
	RecordID    string `json:"record_id"`    // L'ID du document côté Laravel (ex: ORD-123)
	PatientUUID string `json:"patient_uuid"` // Le patient concerné
	AuteurUUID  string `json:"auteur_uuid"`  // Le médecin qui a créé le document
	AuteurOrg   string `json:"auteur_org"`   // L'hôpital de l'auteur
	Hash        string `json:"hash"`         // L'empreinte cryptographique initiale (SHA-256) du document
	Timestamp   string `json:"timestamp"`    // Date de création sécurisée par la blockchain
}

// FinancialTransaction représente la trace immuable d'un reçu ou paiement
type FinancialTransaction struct {
	InvoiceNumber  string `json:"invoice_number"`
	Amount         string `json:"amount"` // String ou float64, ici string pour éviter les problèmes de précision
	Currency       string `json:"currency"`
	OrganizationID string `json:"organization_id"`
	Hash           string `json:"hash"` // Le hash généré par Laravel garantissant l'intégrité
	Timestamp      string `json:"timestamp"`
}
