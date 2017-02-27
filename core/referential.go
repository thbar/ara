package core

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/af83/edwig/logger"
	"github.com/af83/edwig/model"
)

type ReferentialId string
type ReferentialSlug string

type Referential struct {
	id   ReferentialId
	slug ReferentialSlug

	Settings map[string]string

	collectManager CollectManagerInterface
	manager        Referentials
	model          model.Model
	modelGuardian  *ModelGuardian
	partners       Partners
	startedAt      time.Time
}

type Referentials interface {
	model.Startable

	New(slug ReferentialSlug) *Referential
	Find(id ReferentialId) *Referential
	FindBySlug(slug ReferentialSlug) *Referential
	FindAll() []*Referential
	Save(stopArea *Referential) bool
	Delete(stopArea *Referential) bool
	Load() error
}

var referentials = NewMemoryReferentials()

type APIReferential struct {
	id       ReferentialId
	Slug     ReferentialSlug   `json:"Slug,omitempty"`
	Errors   Errors            `json:"Errors,omitempty"`
	Settings map[string]string `json:"Settings,omitempty"`

	manager Referentials
}

func (referential *APIReferential) Id() ReferentialId {
	return referential.id
}

func (referential *APIReferential) Validate() bool {
	referential.Errors = NewErrors()

	if referential.Slug == "" {
		referential.Errors.Add("Slug", ERROR_BLANK)
	}

	// Check Slug uniqueness
	for _, existingReferential := range referential.manager.FindAll() {
		if existingReferential.id != referential.Id() {
			if referential.Slug == existingReferential.slug {
				referential.Errors.Add("Slug", ERROR_UNIQUE)
			}
		}
	}

	return len(referential.Errors) == 0
}

func (referential *Referential) Id() ReferentialId {
	return referential.id
}

func (referential *Referential) Slug() ReferentialSlug {
	return referential.slug
}

func (referential *Referential) Setting(key string) string {
	return referential.Settings[key]
}

func (referential *Referential) StartedAt() time.Time {
	return referential.startedAt
}

// WIP: Interface ?
func (referential *Referential) CollectManager() CollectManagerInterface {
	return referential.collectManager
}

func (referential *Referential) Model() model.Model {
	return referential.model
}

func (referential *Referential) ModelGuardian() *ModelGuardian {
	return referential.modelGuardian
}

func (referential *Referential) Partners() Partners {
	return referential.partners
}

func (referential *Referential) Start() {
	referential.startedAt = model.DefaultClock().Now()

	referential.partners.Start()
	referential.modelGuardian.Start()
}

func (referential *Referential) Stop() {
	referential.partners.Stop()
	referential.modelGuardian.Stop()
}

func (referential *Referential) Save() (ok bool) {
	ok = referential.manager.Save(referential)
	return
}

func (referential *Referential) NewTransaction() *model.Transaction {
	return model.NewTransaction(referential.model)
}

func (referential *Referential) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"Id":       referential.id,
		"Slug":     referential.slug,
		"Settings": referential.Settings,
		"Partners": referential.partners,
	})
}

func (referential *Referential) Definition() *APIReferential {
	return &APIReferential{
		id:       referential.id,
		Slug:     referential.slug,
		Settings: referential.Settings,
		Errors:   NewErrors(),
		manager:  referential.manager,
	}
}

func (referential *Referential) SetDefinition(apiReferential *APIReferential) {
	//referential.id = apiReferential.Id
	referential.slug = apiReferential.Slug
	referential.Settings = apiReferential.Settings
}

func (referential *Referential) NextReloadAt() time.Time {
	return referential.model.GetDate()
}

func (referential *Referential) ReloadModel() {
	referential.modelGuardian.Reload()
}

type MemoryReferentials struct {
	model.UUIDConsumer

	byId map[ReferentialId]*Referential
}

func NewMemoryReferentials() *MemoryReferentials {
	return &MemoryReferentials{
		byId: make(map[ReferentialId]*Referential),
	}
}

func CurrentReferentials() Referentials {
	return referentials
}

func (manager *MemoryReferentials) New(slug ReferentialSlug) *Referential {
	referential := manager.new()
	referential.slug = slug
	return referential
}

func (manager *MemoryReferentials) new() *Referential {
	model := model.NewMemoryModel()

	referential := &Referential{
		manager:  manager,
		model:    model,
		Settings: make(map[string]string),
	}

	referential.partners = NewPartnerManager(referential)
	referential.collectManager = NewCollectManager(referential.partners)

	referential.modelGuardian = NewModelGuardian(referential)
	return referential
}

func (manager *MemoryReferentials) Find(id ReferentialId) *Referential {
	referential, _ := manager.byId[id]
	return referential
}

func (manager *MemoryReferentials) FindBySlug(slug ReferentialSlug) *Referential {
	for _, referential := range manager.byId {
		if referential.slug == slug {
			return referential
		}
	}
	return nil
}

func (manager *MemoryReferentials) FindAll() (referentials []*Referential) {
	if len(manager.byId) == 0 {
		return []*Referential{}
	}
	for _, referential := range manager.byId {
		referentials = append(referentials, referential)
	}
	return
}

func (manager *MemoryReferentials) Save(referential *Referential) bool {
	if referential.id == "" {
		referential.id = ReferentialId(manager.NewUUID())
	}
	referential.manager = manager
	referential.collectManager.HandleStopVisitUpdateEvent(model.NewStopVisitUpdateManager(referential.model))
	manager.byId[referential.id] = referential
	return true
}

func (manager *MemoryReferentials) Delete(referential *Referential) bool {
	delete(manager.byId, referential.id)
	return true
}

func (manager *MemoryReferentials) Load() error {
	var selectReferentials []struct {
		Referential_id string
		Slug           string
		Settings       sql.NullString
	}

	_, err := model.Database.Select(&selectReferentials, "select * from referentials")
	if err != nil {
		return err
	}

	for _, r := range selectReferentials {
		referential := manager.new()
		referential.id = ReferentialId(r.Referential_id)
		referential.slug = ReferentialSlug(r.Slug)

		referential.Partners().Load()

		if r.Settings.Valid && len(r.Settings.String) > 0 {
			if err = json.Unmarshal([]byte(r.Settings.String), &referential.Settings); err != nil {
				return err
			}
		}

		manager.Save(referential)
	}

	logger.Log.Debugf("Loaded Referentials from database")
	return nil
}

func (manager *MemoryReferentials) Start() {
	for _, referential := range manager.byId {
		referential.Start()
	}
}

type ReferentialsConsumer struct {
	referentials Referentials
}

func (consumer *ReferentialsConsumer) SetReferentials(referentials Referentials) {
	consumer.referentials = referentials
}

func (consumer *ReferentialsConsumer) CurrentReferentials() Referentials {
	if consumer.referentials == nil {
		consumer.referentials = CurrentReferentials()
	}
	return consumer.referentials
}
