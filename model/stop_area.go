package model

import (
	"encoding/json"
	"time"
)

type StopAreaId string

type StopArea struct {
	ObjectIDConsumer
	model Model

	id          StopAreaId
	requestedAt time.Time
	updatedAt   time.Time

	Name string
	// ...
}

func NewStopArea(model Model) *StopArea {
	stopArea := &StopArea{model: model}
	stopArea.objectids = make(ObjectIDs)
	return stopArea
}

func (stopArea *StopArea) Id() StopAreaId {
	return stopArea.id
}

func (stopArea *StopArea) RequestedAt() time.Time {
	return stopArea.requestedAt
}

func (stopArea *StopArea) Requested(requestTime time.Time) {
	stopArea.requestedAt = requestTime
}

func (stopArea *StopArea) UpdatedAt() time.Time {
	return stopArea.updatedAt
}

func (stopArea *StopArea) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"Id":   stopArea.id,
		"Name": stopArea.Name,
	})
}

func (stopArea *StopArea) UnmarshalJSON(data []byte) error {
	aux := &struct {
		Name      string
		ObjectIDs ObjectIDs
	}{
		ObjectIDs: make(ObjectIDs),
	}
	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	stopArea.Name = aux.Name
	stopArea.ObjectIDConsumer.objectids = aux.ObjectIDs

	return nil
}

func (stopArea *StopArea) Save() (ok bool) {
	ok = stopArea.model.StopAreas().Save(stopArea)
	return
}

type MemoryStopAreas struct {
	UUIDConsumer

	model Model

	byIdentifier map[StopAreaId]*StopArea
	byObjectId   map[string]map[string]StopAreaId
}

type StopAreas interface {
	UUIDInterface

	New() StopArea
	Find(id StopAreaId) (StopArea, bool)
	FindByObjectId(objectid ObjectID) (StopArea, bool)
	FindAll() []StopArea
	Save(stopArea *StopArea) bool
	Delete(stopArea *StopArea) bool
}

func NewMemoryStopAreas() *MemoryStopAreas {
	return &MemoryStopAreas{
		byIdentifier: make(map[StopAreaId]*StopArea),
		byObjectId:   make(map[string]map[string]StopAreaId),
	}
}

func (manager *MemoryStopAreas) New() StopArea {
	stopArea := NewStopArea(manager.model)
	return *stopArea
}

func (manager *MemoryStopAreas) Find(id StopAreaId) (StopArea, bool) {
	stopArea, ok := manager.byIdentifier[id]
	if ok {
		return *stopArea, true
	} else {
		return StopArea{}, false
	}
}

func (manager *MemoryStopAreas) FindByObjectId(objectid ObjectID) (StopArea, bool) {
	valueMap, ok := manager.byObjectId[objectid.Kind()]
	if !ok {
		return StopArea{}, false
	}
	id, ok := valueMap[objectid.Value()]
	if !ok {
		return StopArea{}, false
	}
	return *manager.byIdentifier[id], true
}

func (manager *MemoryStopAreas) FindAll() (stopAreas []StopArea) {
	if len(manager.byIdentifier) == 0 {
		return []StopArea{}
	}
	for _, stopArea := range manager.byIdentifier {
		stopAreas = append(stopAreas, *stopArea)
	}
	return
}

func (manager *MemoryStopAreas) Save(stopArea *StopArea) bool {
	if stopArea.Id() == "" {
		stopArea.id = StopAreaId(manager.NewUUID())
	}
	stopArea.model = manager.model
	manager.byIdentifier[stopArea.Id()] = stopArea
	for _, objectid := range stopArea.ObjectIDs() {
		_, ok := manager.byObjectId[objectid.Kind()]
		if !ok {
			manager.byObjectId[objectid.Kind()] = make(map[string]StopAreaId)
		}
		manager.byObjectId[objectid.Kind()][objectid.Value()] = stopArea.Id()
	}
	return true
}

func (manager *MemoryStopAreas) Delete(stopArea *StopArea) bool {
	delete(manager.byIdentifier, stopArea.Id())
	for _, objectid := range stopArea.ObjectIDs() {
		valueMap := manager.byObjectId[objectid.Kind()]
		delete(valueMap, objectid.Value())
	}
	return true
}
