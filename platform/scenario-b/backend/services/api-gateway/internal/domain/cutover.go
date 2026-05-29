// Package domain defines Scenario B cutover inventory models for the api-gateway.
package domain

import "time"

// CutoverStatus enumerates the lifecycle states of the API cutover plan.
type CutoverStatus string

const (
	CutoverPlanned    CutoverStatus = "PLANNED"
	CutoverInProgress CutoverStatus = "IN_PROGRESS"
	CutoverCompleted  CutoverStatus = "COMPLETED"
	CutoverRolledBack CutoverStatus = "ROLLED_BACK"
)

// ScenarioBApiCutoverPlan represents the single big-bang cutover plan from API v1 to v2 (FR-016).
type ScenarioBApiCutoverPlan struct {
	CutoverID               string        `gorm:"primaryKey;column:cutover_id;type:varchar(64)"`
	Status                  CutoverStatus `gorm:"column:status;not null;default:'PLANNED'"`
	ScheduledAt             time.Time     `gorm:"column:scheduled_at;not null"`
	ExecutedAt              *time.Time    `gorm:"column:executed_at"`
	LegacyArtifactsTotal    int           `gorm:"column:legacy_artifacts_total;not null;default:0"`
	LegacyArtifactsRemoved  int           `gorm:"column:legacy_artifacts_removed;not null;default:0"`
	InfraComponentsReused   string        `gorm:"column:infra_components_reused;type:jsonb"` // JSON array of component IDs
	CreatedAt               time.Time     `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt               time.Time     `gorm:"column:updated_at;autoUpdateTime"`
}

// EndpointDomain enumerates the domain categories of the v2 API.
type EndpointDomain string

const (
	EndpointDomainQuote           EndpointDomain = "QUOTE"
	EndpointDomainSwap            EndpointDomain = "SWAP"
	EndpointDomainPoolStatus      EndpointDomain = "POOL_STATUS"
	EndpointDomainGovernanceRisk  EndpointDomain = "GOVERNANCE_RISK"
	EndpointDomainBridge          EndpointDomain = "BRIDGE"
	EndpointDomainCompliance      EndpointDomain = "COMPLIANCE"
	EndpointDomainOversight       EndpointDomain = "OVERSIGHT"
)

// EndpointState enumerates the lifecycle state of an API endpoint.
type EndpointState string

const (
	EndpointActive     EndpointState = "ACTIVE"
	EndpointDeprecated EndpointState = "DEPRECATED"
	EndpointRemoved    EndpointState = "REMOVED"
)

// ScenarioBEndpointContract defines the functional contract of each v2 API endpoint.
type ScenarioBEndpointContract struct {
	EndpointID            string         `gorm:"primaryKey;column:endpoint_id;type:varchar(64)"`
	Domain                EndpointDomain `gorm:"column:domain;not null"`
	Path                  string         `gorm:"column:path;not null"`
	Method                string         `gorm:"column:method;not null"`
	State                 EndpointState  `gorm:"column:state;not null;default:'ACTIVE'"`
	AcceptanceCriteriaRef string         `gorm:"column:acceptance_criteria_ref"`
	CreatedAt             time.Time      `gorm:"column:created_at;autoCreateTime"`
}

// ArtifactType enumerates the type of a legacy artifact.
type ArtifactType string

const (
	ArtifactRoute   ArtifactType = "ROUTE"
	ArtifactHandler ArtifactType = "HANDLER"
	ArtifactService ArtifactType = "SERVICE"
	ArtifactJob     ArtifactType = "JOB"
	ArtifactTest    ArtifactType = "TEST"
	ArtifactDoc     ArtifactType = "DOC"
	ArtifactSchema  ArtifactType = "SCHEMA"
)

// MigrationAction enumerates the cutover action for a legacy artifact.
type MigrationAction string

const (
	MigrationRemove  MigrationAction = "REMOVE"
	MigrationReplace MigrationAction = "REPLACE"
)

// LegacyArtifactInventory tracks each Scenario A artifact scheduled for removal or replacement.
type LegacyArtifactInventory struct {
	ArtifactID     string          `gorm:"primaryKey;column:artifact_id;type:varchar(64)"`
	ArtifactType   ArtifactType    `gorm:"column:artifact_type;not null"`
	SourcePath     string          `gorm:"column:source_path;not null"`
	MigrationAction MigrationAction `gorm:"column:migration_action;not null"`
	ReplacementRef string          `gorm:"column:replacement_ref"`
	CutoverBatch   string          `gorm:"column:cutover_batch"`
	CreatedAt      time.Time       `gorm:"column:created_at;autoCreateTime"`
}

// InfraLayer enumerates the infrastructure layer of a reused component.
type InfraLayer string

const (
	InfraLayerIdentity     InfraLayer = "IDENTITY"
	InfraLayerDatastore    InfraLayer = "DATASTORE"
	InfraLayerCache        InfraLayer = "CACHE"
	InfraLayerRuntime      InfraLayer = "RUNTIME"
	InfraLayerObservability InfraLayer = "OBSERVABILITY"
)

// ReuseMode enumerates how the component is reused.
type ReuseMode string

const (
	ReuseModeAsIs        ReuseMode = "AS_IS"
	ReuseModeReconfigured ReuseMode = "RECONFIGURED"
)

// InfrastructureReuseRegister records transversal infrastructure reused from Scenario A.
type InfrastructureReuseRegister struct {
	ComponentID           string     `gorm:"primaryKey;column:component_id;type:varchar(64)"`
	ComponentName         string     `gorm:"column:component_name;not null"`
	Layer                 InfraLayer `gorm:"column:layer;not null"`
	ReuseMode             ReuseMode  `gorm:"column:reuse_mode;not null"`
	LegacyDependencyCheck bool       `gorm:"column:legacy_dependency_check;not null;default:false"`
	CreatedAt             time.Time  `gorm:"column:created_at;autoCreateTime"`
}
