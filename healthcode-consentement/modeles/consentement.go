package modele

import (
	"time"
)

type TypeBeneficiaire string

// Enumeration des types de beneficiaire d'acces
const (
	Medecin       TypeBeneficiaire = "MEDECIN"
	Specialiste   TypeBeneficiaire = "SPECIALISTE"
	Etablissement TypeBeneficiaire = "ETABLISSEMENT"
	Labo          TypeBeneficiaire = "LABORATOIRE"
	Pharmacie     TypeBeneficiaire = "PHARMACIE"
	Urgence       TypeBeneficiaire = "URGENCE"
	Individu      TypeBeneficiaire = "INDIVIDU"
	Autre         TypeBeneficiaire = "AUTRE"
)

type TypeConsentement string

// Enumeration des types de consentement
const (
	Permanent  TypeConsentement = "PERMANENT"
	Temporaire TypeConsentement = "TEMPORAIRE"
	Ponctuel   TypeConsentement = "PONCTUEL"
	Urgent     TypeConsentement = "URGENT"
)

type StatutConsentement string

// Enumeration des statuts de consentement
const (
	Actif    StatutConsentement = "ACTIF"
	Expire   StatutConsentement = "EXPIRE"
	Revoque  StatutConsentement = "REVOQUE"
	Suspendu StatutConsentement = "SUSPENDU"
)

// Precision sur le type d'acces
type PrecisionConsentement struct {
	DossierComplet            bool `json:"dossier_complet"`
	Prescriptions             bool `json:"prescriptions"`
	Imagerie                  bool `json:"imagerie"`
	Biologie                  bool `json:"biologie"`
	Psychiatrique             bool `json:"psychiatrique"`
	Addictologie              bool `json:"addictologie"`
	Antecedents               bool `json:"antecedents"`
	Vaccinations              bool `json:"vaccinations"`
	UrgencesHistorique        bool `json:"urgences_historique"`
	HistoriqueHospitalisation bool `json:"historique_hospitalisation"`
	Autre                     bool `json:"autre"`
}

// Structure d'un consentement
type Consentement struct {
	ConsentementID             string                `json:"consentement_id"`
	PatientUUID                string                `json:"patient_uuid"`
	PatientCertHash            string                `json:"patient_cert_hash"`
	BeneficiaireType           TypeBeneficiaire      `json:"beneficiaire_type"`
	BeneficiaireUUID           string                `json:"beneficiaire_uuid"`
	BeneficiaireOrg            string                `json:"beneficiaire_org"`
	BeneficiairePrecisionAcces PrecisionConsentement `json:"beneficiaire_precision_acces"`
	TypeConsentement           TypeConsentement      `json:"type_consentement"`
	DateDebut                  string                `json:"date_debut"`
	DateFin                    string                `json:"date_fin"`
	NbAccesMax                 int                   `json:"nb_acces_max"`
	NbAccesUses                int                   `json:"nb_acces_uses"`
	StatutConsentement         StatutConsentement    `json:"statut_consentement"`
	MotifAccorde               string                `json:"motif_accorde"`
	MotifRefus                 string                `json:"motif_refus"`
	DateCreation               string                `json:"date_creation"`
	DateRevocation             string                `json:"date_revocation"`
	SignaturePatient           string                `json:"signature_patient"`
}

// Structure pour l'historique d'un consentement
type HistoriqueConsentement struct {
	TxId      string        `json:"tx_id"`
	IsDelete  bool          `json:"is_delete"`
	Value     *Consentement `json:"value,omitempty"`
	Timestamp string        `json:"timestamp"`
}

// fonction de verification de precision du type d'acces
func (pre *PrecisionConsentement) EstVide() bool {
	return !(pre.DossierComplet || pre.Prescriptions || pre.Imagerie || pre.Biologie || pre.Psychiatrique || pre.Addictologie || pre.Antecedents || pre.Vaccinations || pre.UrgencesHistorique || pre.HistoriqueHospitalisation || pre.Autre)
}

// fonction pour s'assurer que le consentement couvre tous les champs requis
func (pre *PrecisionConsentement) VerifieSiPrecisionSuffisant(requis PrecisionConsentement) bool {
	// Si le patient a accordé un accès au dossier complet, alors toutes les demandes sont automatiquement couvertes
	if pre.DossierComplet {
		return true
	}

	// Si un accès au dossier complet est requis mais que le patient ne l'a pas donné
	if requis.DossierComplet && !pre.DossierComplet {
		return false
	}

	// Pour chaque autre domaine, si c'est requis, cela DOIT être autorisé
	if requis.Prescriptions && !pre.Prescriptions {
		return false
	}
	if requis.Imagerie && !pre.Imagerie {
		return false
	}
	if requis.Biologie && !pre.Biologie {
		return false
	}
	if requis.Psychiatrique && !pre.Psychiatrique {
		return false
	}
	if requis.Addictologie && !pre.Addictologie {
		return false
	}
	if requis.Antecedents && !pre.Antecedents {
		return false
	}
	if requis.Vaccinations && !pre.Vaccinations {
		return false
	}
	if requis.UrgencesHistorique && !pre.UrgencesHistorique {
		return false
	}
	if requis.HistoriqueHospitalisation && !pre.HistoriqueHospitalisation {
		return false
	}
	if requis.Autre && !pre.Autre {
		return false
	}

	// Si aucune des vérifications n'a échoué, alors tous les champs requis sont bien couverts par le consentement
	return true
}

// fonction pour s'assurer que le consentement est valide
func (c Consentement) EstValide() bool {
	dd, _ := time.Parse(time.RFC3339, c.DateDebut)
	var df *time.Time
	if c.DateFin != "" {
		parsedDf, _ := time.Parse(time.RFC3339, c.DateFin)
		df = &parsedDf
	}
	return c.StatutConsentement == Actif && dd.Before(time.Now()) && (df == nil || df.After(time.Now()))
}
