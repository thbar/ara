package core

import (
	"strconv"

	"github.com/af83/edwig/logger"
	"github.com/af83/edwig/model"
)

type StopAreaUpdateSubscriber func(*model.StopAreaUpdateEvent)
type SituationUpdateSubscriber func([]*model.SituationUpdateEvent)

type CollectManagerInterface interface {
	UpdateStopArea(request *StopAreaUpdateRequest)
	HandleStopAreaUpdateEvent(StopAreaUpdateSubscriber)
	BroadcastStopAreaUpdateEvent(event *model.StopAreaUpdateEvent)

	UpdateSituation(request *SituationUpdateRequest)
	HandleSituationUpdateEvent(SituationUpdateSubscriber)
	BroadcastSituationUpdateEvent(event []*model.SituationUpdateEvent)
}

type CollectManager struct {
	model.UUIDConsumer

	StopAreaUpdateSubscribers  []StopAreaUpdateSubscriber
	SituationUpdateSubscribers []SituationUpdateSubscriber
	referential                *Referential
}

// TestCollectManager has a test StopAreaUpdateSubscriber method
type TestCollectManager struct {
	Done            chan bool
	Events          []*model.StopAreaUpdateEvent
	StopVisitEvents []*model.StopVisitUpdateEvent
}

func NewTestCollectManager() CollectManagerInterface {
	return &TestCollectManager{
		Done: make(chan bool, 1),
	}
}

func (manager *TestCollectManager) UpdateStopArea(request *StopAreaUpdateRequest) {
	event := &model.StopAreaUpdateEvent{}
	manager.Events = append(manager.Events, event)

	manager.Done <- true
}

func (manager *TestCollectManager) TestStopAreaUpdateSubscriber(event *model.StopAreaUpdateEvent) {
	for _, stopVisitUpdateEvent := range event.StopVisitUpdateEvents {
		manager.StopVisitEvents = append(manager.StopVisitEvents, stopVisitUpdateEvent)
	}
}

func (manager *TestCollectManager) HandleStopAreaUpdateEvent(StopAreaUpdateSubscriber) {}
func (manager *TestCollectManager) BroadcastStopAreaUpdateEvent(event *model.StopAreaUpdateEvent) {
	manager.Events = append(manager.Events, event)
}

func (manager *TestCollectManager) UpdateSituation(*SituationUpdateRequest)              {}
func (manager *TestCollectManager) HandleSituationUpdateEvent(SituationUpdateSubscriber) {}
func (manager *TestCollectManager) BroadcastSituationUpdateEvent(event []*model.SituationUpdateEvent) {
}

// TEST END

func NewCollectManager(referential *Referential) CollectManagerInterface {
	return &CollectManager{
		referential:                referential,
		StopAreaUpdateSubscribers:  make([]StopAreaUpdateSubscriber, 0),
		SituationUpdateSubscribers: make([]SituationUpdateSubscriber, 0),
	}
}

func (manager *CollectManager) HandleStopAreaUpdateEvent(StopAreaUpdateSubscriber StopAreaUpdateSubscriber) {
	manager.StopAreaUpdateSubscribers = append(manager.StopAreaUpdateSubscribers, StopAreaUpdateSubscriber)
}

func (manager *CollectManager) BroadcastStopAreaUpdateEvent(event *model.StopAreaUpdateEvent) {
	for _, StopAreaUpdateSubscriber := range manager.StopAreaUpdateSubscribers {
		StopAreaUpdateSubscriber(event)
	}
}

func (manager *CollectManager) UpdateStopArea(request *StopAreaUpdateRequest) {
	stopArea, ok := manager.referential.Model().StopAreas().Find(request.StopAreaId())
	if !ok {
		logger.Log.Debugf("Can't find StopArea %v in Collect Manager", request.StopAreaId())
		return
	}
	partner := manager.bestPartner(stopArea)
	if partner != nil {
		// Check if StopArea is not monitored to send an event
		if !stopArea.Monitored {
			logger.Log.Debugf("Found a partner for StopArea %v in Collect Manager", request.StopAreaId())
			stopAreaUpdateEvent := model.NewStopAreaMonitoredEvent(manager.NewUUID(), stopArea.Id(), true)
			manager.BroadcastStopAreaUpdateEvent(stopAreaUpdateEvent)
		}
		manager.requestStopAreaUpdate(partner, request)
		return
	}
	logger.Log.Debugf("Can't find a partner for StopArea %v in Collect Manager", request.StopAreaId())
	// Do nothing if StopArea is already not monitored
	if !stopArea.Monitored {
		return
	}
	stopAreaUpdateEvent := model.NewStopAreaMonitoredEvent(manager.NewUUID(), stopArea.Id(), false)
	manager.BroadcastStopAreaUpdateEvent(stopAreaUpdateEvent)
}

func (manager *CollectManager) bestPartner(stopArea model.StopArea) *Partner {
	for _, partner := range manager.referential.Partners().FindAllByCollectPriority() {
		if partner.PartnerStatus.OperationnalStatus != OPERATIONNAL_STATUS_UP {
			continue
		}
		_, connectorPresent := partner.Connector(SIRI_STOP_MONITORING_REQUEST_COLLECTOR)
		_, testConnectorPresent := partner.Connector(TEST_STOP_MONITORING_REQUEST_COLLECTOR)
		_, subscriptionPresent := partner.Connector(SIRI_STOP_MONITORING_SUBSCRIPTION_COLLECTOR)

		if !(connectorPresent || testConnectorPresent || subscriptionPresent) {
			continue
		}

		partnerKind := partner.Setting("remote_objectid_kind")

		stopAreaObjectID, ok := stopArea.ObjectID(partnerKind)
		if !ok {
			continue
		}

		lineIds := make(map[string]struct{})
		for _, lineId := range stopArea.LineIds {
			line, ok := manager.referential.Model().Lines().Find(lineId)
			if !ok {
				continue
			}
			lineObjectID, ok := line.ObjectID(partnerKind)
			if !ok {
				continue
			}
			lineIds[lineObjectID.Value()] = struct{}{}
		}

		if partner.CanCollect(stopAreaObjectID, lineIds) {
			return partner
		}
	}
	return nil
}

func (manager *CollectManager) requestStopAreaUpdate(partner *Partner, request *StopAreaUpdateRequest) {
	logger.Log.Debugf("RequestStopAreaUpdate %v", request.StopAreaId())

	if collect := partner.StopMonitoringSubscriptionCollector(); collect != nil {
		collect.RequestStopAreaUpdate(request)
		return
	}
	partner.StopMonitoringRequestCollector().RequestStopAreaUpdate(request)
}

func (manager *CollectManager) HandleSituationUpdateEvent(SituationUpdateSubscriber SituationUpdateSubscriber) {
	manager.SituationUpdateSubscribers = append(manager.SituationUpdateSubscribers, SituationUpdateSubscriber)
}

func (manager *CollectManager) BroadcastSituationUpdateEvent(event []*model.SituationUpdateEvent) {
	for _, SituationUpdateSubscriber := range manager.SituationUpdateSubscribers {
		SituationUpdateSubscriber(event)
	}
}

func (manager *CollectManager) UpdateSituation(request *SituationUpdateRequest) {
	switch request.Kind() {
	case SITUATION_UPDATE_REQUEST_ALL:
		manager.requestAllSituations()
	case SITUATION_UPDATE_REQUEST_LINE:
		manager.requestLineFilteredSituation(request.RequestedId())
	case SITUATION_UPDATE_REQUEST_STOP_AREA:
		manager.requestStopAreaFilteredSituation(request.RequestedId())
	default:
		logger.Log.Debugf("SituationUpdateRequest of unknown kind")
	}
}

func (manager *CollectManager) requestAllSituations() {
	for _, partner := range manager.referential.Partners().FindAllByCollectPriority() {
		if partner.PartnerStatus.OperationnalStatus != OPERATIONNAL_STATUS_UP {
			continue
		}
		if b, _ := strconv.ParseBool(partner.Setting("collect.filter_general_messages")); b {
			continue
		}

		requestConnector := partner.GeneralMessageRequestCollector()
		subscriptionConnector := partner.GeneralMessageSubscriptionCollector()
		if requestConnector == nil && subscriptionConnector == nil {
			continue
		}

		logger.Log.Debugf("RequestAllSituationsUpdate for Partner %v", partner.Slug())
		if subscriptionConnector != nil {
			subscriptionConnector.RequestAllSituationsUpdate()
			continue
		}
		requestConnector.RequestSituationUpdate(SITUATION_UPDATE_REQUEST_ALL, "")
	}
}

func (manager *CollectManager) requestLineFilteredSituation(requestedId string) {
	line, ok := manager.referential.Model().Lines().Find(model.LineId(requestedId))
	if !ok {
		logger.Log.Debugf("Can't find Line to request %v", requestedId)
		return
	}

	for _, partner := range manager.referential.Partners().FindAllByCollectPriority() {
		if partner.PartnerStatus.OperationnalStatus != OPERATIONNAL_STATUS_UP {
			continue
		}
		if b, _ := strconv.ParseBool(partner.Setting("collect.filter_general_messages")); !b {
			continue
		}

		requestConnector := partner.GeneralMessageRequestCollector()
		subscriptionConnector := partner.GeneralMessageSubscriptionCollector()

		if requestConnector == nil && subscriptionConnector == nil {
			continue
		}

		partnerKind := partner.Setting("remote_objectid_kind")

		lineObjectID, ok := line.ObjectID(partnerKind)
		if !ok {
			continue
		}

		if !partner.CanCollectLine(lineObjectID) {
			continue
		}

		logger.Log.Debugf("RequestSituationUpdate %v with Partner %v", lineObjectID.Value(), partner.Slug())
		if subscriptionConnector != nil {
			subscriptionConnector.RequestSituationUpdate(SITUATION_UPDATE_REQUEST_LINE, lineObjectID)
			return
		}
		requestConnector.RequestSituationUpdate(SITUATION_UPDATE_REQUEST_LINE, lineObjectID.Value())
		return
	}
	logger.Log.Debugf("Can't find a partner to request filtered Situations for Line %v", requestedId)
}

func (manager *CollectManager) requestStopAreaFilteredSituation(requestedId string) {
	stopArea, ok := manager.referential.Model().StopAreas().Find(model.StopAreaId(requestedId))
	if !ok {
		logger.Log.Debugf("Can't find StopArea to request %v", requestedId)
		return
	}

	for _, partner := range manager.referential.Partners().FindAllByCollectPriority() {
		if partner.PartnerStatus.OperationnalStatus != OPERATIONNAL_STATUS_UP {
			continue
		}
		if b, _ := strconv.ParseBool(partner.Setting("collect.filter_general_messages")); !b {
			continue
		}

		requestConnector := partner.GeneralMessageRequestCollector()
		subscriptionConnector := partner.GeneralMessageSubscriptionCollector()

		if requestConnector == nil && subscriptionConnector == nil {
			continue
		}

		partnerKind := partner.Setting("remote_objectid_kind")

		stopAreaObjectID, ok := stopArea.ObjectID(partnerKind)
		if !ok {
			continue
		}

		lineIds := make(map[string]struct{})
		for _, lineId := range stopArea.LineIds {
			line, ok := manager.referential.Model().Lines().Find(lineId)
			if !ok {
				continue
			}
			lineObjectID, ok := line.ObjectID(partnerKind)
			if !ok {
				continue
			}
			lineIds[lineObjectID.Value()] = struct{}{}
		}

		if !partner.CanCollect(stopAreaObjectID, lineIds) {
			continue
		}

		logger.Log.Debugf("RequestSituationUpdate %v with Partner %v", stopAreaObjectID.Value(), partner.Slug())
		if subscriptionConnector != nil {
			subscriptionConnector.RequestSituationUpdate(SITUATION_UPDATE_REQUEST_STOP_AREA, stopAreaObjectID)
			return
		}
		requestConnector.RequestSituationUpdate(SITUATION_UPDATE_REQUEST_STOP_AREA, stopAreaObjectID.Value())
		return
	}
	logger.Log.Debugf("Can't find a partner to request filtered Situations for StopArea %v", requestedId)
}
