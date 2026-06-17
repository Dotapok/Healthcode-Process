package modeles

type StatutAcces string

const (
	EnAttente StatutAcces = "EN_ATTENTE"
	Accorde   StatutAcces = "ACCORDE"
	Refuse    StatutAcces = "REFUSE"
)

// DemandeAccesAsset représente une session de requête formulée par un médecin
type DemandeAcces struct {
	DemandeID             string      `json:"demande_id"`
	MedecinUUID           string      `json:"medecin_uuid"`
	PatientUUID           string      `json:"patient_uuid"`
	ConsentementID        string      `json:"consentement_id"`
	PrecisionNiveauRequis string      `json:"precision_niveau_requis"` // JSON chaîné du niveau d'accès demandé
	Etablissement         string      `json:"etablissement"`
	Statut                StatutAcces `json:"statut"`
	DateDemande           string      `json:"date_demande"`
	DateExpiration        string      `json:"date_expiration"` // Session éphémère (ex: valide 4h)
}
