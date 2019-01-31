package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/af83/edwig/audit"
	"github.com/af83/edwig/logger"
	"github.com/af83/edwig/model"
	"github.com/af83/edwig/siri"
)

type EstimatedTimetableBroadcaster interface {
	RequestLine(request *siri.XMLGetEstimatedTimetable) *siri.SIRIEstimatedTimeTableResponse
}

type SIRIEstimatedTimetableBroadcaster struct {
	model.ClockConsumer

	siriConnector
}

type SIRIEstimatedTimetableBroadcasterFactory struct{}

func NewSIRIEstimatedTimetableBroadcaster(partner *Partner) *SIRIEstimatedTimetableBroadcaster {
	broadcaster := &SIRIEstimatedTimetableBroadcaster{}
	broadcaster.partner = partner
	return broadcaster
}

func (connector *SIRIEstimatedTimetableBroadcaster) RequestLine(request *siri.XMLGetEstimatedTimetable) *siri.SIRIEstimatedTimeTableResponse {
	tx := connector.Partner().Referential().NewTransaction()
	defer tx.Close()

	logStashEvent := connector.newLogStashEvent()
	defer audit.CurrentLogStash().WriteEvent(logStashEvent)

	logXMLEstimatedTimetableRequest(logStashEvent, &request.XMLEstimatedTimetableRequest)
	logStashEvent["requestorRef"] = request.RequestorRef()

	response := &siri.SIRIEstimatedTimeTableResponse{
		Address:                   connector.Partner().Address(),
		ProducerRef:               connector.Partner().ProducerRef(),
		ResponseMessageIdentifier: connector.SIRIPartner().IdentifierGenerator("response_message_identifier").NewMessageIdentifier(),
	}

	response.SIRIEstimatedTimetableDelivery = connector.getEstimatedTimetableDelivery(tx, &request.XMLEstimatedTimetableRequest, logStashEvent)

	logSIRIEstimatedTimetableResponse(logStashEvent, response)

	return response
}

func (connector *SIRIEstimatedTimetableBroadcaster) getEstimatedTimetableDelivery(tx *model.Transaction, request *siri.XMLEstimatedTimetableRequest, logStashEvent audit.LogStashEvent) siri.SIRIEstimatedTimetableDelivery {
	currentTime := connector.Clock().Now()
	monitoringRefs := []string{}
	lineRefs := []string{}

	delivery := siri.SIRIEstimatedTimetableDelivery{
		RequestMessageRef: request.MessageIdentifier(),
		ResponseTimestamp: currentTime,
		Status:            true,
	}

	selectors := []model.StopVisitSelector{}

	if request.PreviewInterval() != 0 {
		duration := request.PreviewInterval()
		now := connector.Clock().Now()
		if !request.StartTime().IsZero() {
			now = request.StartTime()
		}
		selectors = append(selectors, model.StopVisitSelectorByTime(now, now.Add(duration)))
	}
	selector := model.CompositeStopVisitSelector(selectors)

	// SIRIEstimatedJourneyVersionFrame
	for _, lineId := range request.Lines() {
		lineObjectId := model.NewObjectID(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER), lineId)
		line, ok := tx.Model().Lines().FindByObjectId(lineObjectId)
		if !ok {
			logger.Log.Debugf("Cannot find requested line Estimated Time Table with id %v at %v", lineObjectId.String(), connector.Clock().Now())
			continue
		}

		journeyFrame := &siri.SIRIEstimatedJourneyVersionFrame{
			RecordedAtTime: currentTime,
		}

		// SIRIEstimatedVehicleJourney
		for _, vehicleJourney := range tx.Model().VehicleJourneys().FindByLineId(line.Id()) {
			// Handle vehicleJourney Objectid
			vehicleJourneyId, ok := vehicleJourney.ObjectID(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER))
			var datedVehicleJourneyRef string
			if ok {
				datedVehicleJourneyRef = vehicleJourneyId.Value()
			} else {
				defaultObjectID, ok := vehicleJourney.ObjectID("_default")
				if !ok {
					logger.Log.Debugf("Vehicle journey with id %v does not have a proper objectid at %v", vehicleJourneyId, connector.Clock().Now())
					continue
				}
				referenceGenerator := connector.SIRIPartner().IdentifierGenerator("reference_identifier")
				datedVehicleJourneyRef = referenceGenerator.NewIdentifier(IdentifierAttributes{Type: "VehicleJourney", Default: defaultObjectID.Value()})
			}

			estimatedVehicleJourney := &siri.SIRIEstimatedVehicleJourney{
				LineRef:                lineObjectId.Value(),
				DatedVehicleJourneyRef: datedVehicleJourneyRef,
				Attributes:             make(map[string]string),
				References:             make(map[string]model.Reference),
			}
			lineRefs = append(lineRefs, estimatedVehicleJourney.LineRef)
			estimatedVehicleJourney.References = connector.getEstimatedVehicleJourneyReferences(vehicleJourney, tx, vehicleJourney.Origin)
			estimatedVehicleJourney.Attributes = vehicleJourney.Attributes

			// SIRIEstimatedCall
			for _, stopVisit := range tx.Model().StopVisits().FindFollowingByVehicleJourneyId(vehicleJourney.Id()) {
				if !selector(stopVisit) {
					continue
				}

				// Handle StopPointRef
				stopArea, stopAreaId, ok := connector.stopPointRef(stopVisit.StopAreaId, tx)
				if !ok {
					logger.Log.Printf("Ignore StopVisit %v without StopArea or with StopArea without correct ObjectID", stopVisit.Id())
					continue
				}

				connector.resolveOperatorRef(estimatedVehicleJourney.References, stopVisit, tx)

				monitoringRefs = append(monitoringRefs, stopAreaId)
				estimatedCall := &siri.SIRIEstimatedCall{
					ArrivalStatus:      string(stopVisit.ArrivalStatus),
					DepartureStatus:    string(stopVisit.DepartureStatus),
					AimedArrivalTime:   stopVisit.Schedules.Schedule("aimed").ArrivalTime(),
					AimedDepartureTime: stopVisit.Schedules.Schedule("aimed").DepartureTime(),
					Order:              stopVisit.PassageOrder,
					StopPointRef:       stopAreaId,
					StopPointName:      stopArea.Name,
					DestinationDisplay: stopVisit.Attributes["DestinationDisplay"],
					VehicleAtStop:      stopVisit.VehicleAtStop,
				}

				if stopArea.Monitored {
					estimatedCall.ExpectedArrivalTime = stopVisit.Schedules.Schedule("expected").ArrivalTime()
					estimatedCall.ExpectedDepartureTime = stopVisit.Schedules.Schedule("expected").DepartureTime()
				} else {
					delivery.Status = false
					delivery.ErrorType = "OtherError"
					delivery.ErrorNumber = 1
					delivery.ErrorText = fmt.Sprintf("Erreur [PRODUCER_UNAVAILABLE] : %v indisponible", strings.Join(stopArea.Origins.PartnersKO(), ", "))
				}

				estimatedVehicleJourney.EstimatedCalls = append(estimatedVehicleJourney.EstimatedCalls, estimatedCall)
			}
			if len(estimatedVehicleJourney.EstimatedCalls) != 0 {
				journeyFrame.EstimatedVehicleJourneys = append(journeyFrame.EstimatedVehicleJourneys, estimatedVehicleJourney)
			}
		}
		if len(journeyFrame.EstimatedVehicleJourneys) != 0 {
			delivery.EstimatedJourneyVersionFrames = append(delivery.EstimatedJourneyVersionFrames, journeyFrame)
		}
	}

	logSIRIEstimatedTimetableDelivery(logStashEvent, delivery, monitoringRefs, lineRefs)

	return delivery
}

func (connector *SIRIEstimatedTimetableBroadcaster) stopPointRef(stopAreaId model.StopAreaId, tx *model.Transaction) (model.StopArea, string, bool) {
	stopPointRef, ok := tx.Model().StopAreas().Find(stopAreaId)
	if !ok {
		return model.StopArea{}, "", false
	}
	stopPointRefObjectId, ok := stopPointRef.ObjectID(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER))
	if ok {
		return stopPointRef, stopPointRefObjectId.Value(), true
	}
	referent, ok := stopPointRef.Referent()
	if ok {
		referentObjectId, ok := referent.ObjectID(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER))
		if ok {
			return referent, referentObjectId.Value(), true
		}
	}
	return model.StopArea{}, "", false
}

func (connector *SIRIEstimatedTimetableBroadcaster) getEstimatedVehicleJourneyReferences(vehicleJourney model.VehicleJourney, tx *model.Transaction, origin string) map[string]model.Reference {
	references := make(map[string]model.Reference)

	for _, refType := range []string{"OriginRef", "DestinationRef"} {
		ref, ok := vehicleJourney.Reference(refType)
		if !ok || ref == (model.Reference{}) || ref.ObjectId == nil {
			continue
		}
		if refType == "DestinationRef" && connector.noDestinationRefRewrite(origin) {
			references[refType] = ref
			continue
		}
		if foundStopArea, ok := tx.Model().StopAreas().FindByObjectId(*ref.ObjectId); ok {
			obj, ok := foundStopArea.ReferentOrSelfObjectId(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER))
			if ok {
				references[refType] = *model.NewReference(obj)
				continue
			}
		}
		generator := connector.SIRIPartner().IdentifierGenerator("reference_stop_area_identifier")
		defaultObjectID := model.NewObjectID(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER), generator.NewIdentifier(IdentifierAttributes{Default: ref.GetSha1()}))
		references[refType] = *model.NewReference(defaultObjectID)
	}

	return references
}

func (connector *SIRIEstimatedTimetableBroadcaster) noDestinationRefRewrite(origin string) bool {
	for _, o := range connector.Partner().NoDestinationRefRewritingFrom() {
		if origin == strings.TrimSpace(o) {
			return true
		}
	}
	return false
}

func (connector *SIRIEstimatedTimetableBroadcaster) resolveOperatorRef(refs map[string]model.Reference, stopVisit model.StopVisit, tx *model.Transaction) {
	if _, ok := refs["OperatorRef"]; ok {
		return
	}

	operatorRef, ok := stopVisit.Reference("OperatorRef")
	if !ok || operatorRef == (model.Reference{}) || operatorRef.ObjectId == nil {
		return
	}
	operator, ok := tx.Model().Operators().FindByObjectId(*operatorRef.ObjectId)
	if !ok {
		refs["OperatorRef"] = *model.NewReference(*operatorRef.ObjectId)
		return
	}
	obj, ok := operator.ObjectID(connector.partner.RemoteObjectIDKind(SIRI_ESTIMATED_TIMETABLE_REQUEST_BROADCASTER))
	if !ok {
		refs["OperatorRef"] = *model.NewReference(*operatorRef.ObjectId)
		return
	}
	refs["OperatorRef"] = *model.NewReference(obj)
}

func (connector *SIRIEstimatedTimetableBroadcaster) newLogStashEvent() audit.LogStashEvent {
	event := connector.partner.NewLogStashEvent()
	event["connector"] = "EstimatedTimetableRequestBroadcaster"
	return event
}

func (factory *SIRIEstimatedTimetableBroadcasterFactory) Validate(apiPartner *APIPartner) bool {
	ok := apiPartner.ValidatePresenceOfSetting("remote_objectid_kind")
	ok = ok && apiPartner.ValidatePresenceOfSetting("local_credential")
	return ok
}

func (factory *SIRIEstimatedTimetableBroadcasterFactory) CreateConnector(partner *Partner) Connector {
	return NewSIRIEstimatedTimetableBroadcaster(partner)
}

func logXMLEstimatedTimetableRequest(logStashEvent audit.LogStashEvent, request *siri.XMLEstimatedTimetableRequest) {
	logStashEvent["siriType"] = "EstimatedTimetableResponse"
	logStashEvent["messageIdentifier"] = request.MessageIdentifier()
	logStashEvent["requestTimestamp"] = request.RequestTimestamp().String()
	logStashEvent["requestedLines"] = strings.Join(request.Lines(), ",")
	logStashEvent["requestXML"] = request.RawXML()
}

func logSIRIEstimatedTimetableDelivery(logStashEvent audit.LogStashEvent, delivery siri.SIRIEstimatedTimetableDelivery, monitoringRefs, lineRefs []string) {
	logStashEvent["requestMessageRef"] = delivery.RequestMessageRef
	logStashEvent["responseTimestamp"] = delivery.ResponseTimestamp.String()
	logStashEvent["monitoringRefs"] = strings.Join(monitoringRefs, ",")
	logStashEvent["status"] = strconv.FormatBool(delivery.Status)
	if !delivery.Status {
		logStashEvent["errorType"] = delivery.ErrorType
		if delivery.ErrorType == "OtherError" {
			logStashEvent["errorNumber"] = strconv.Itoa(delivery.ErrorNumber)
		}
		logStashEvent["errorText"] = delivery.ErrorText
	}
}

func logSIRIEstimatedTimetableResponse(logStashEvent audit.LogStashEvent, response *siri.SIRIEstimatedTimeTableResponse) {
	logStashEvent["address"] = response.Address
	logStashEvent["producerRef"] = response.ProducerRef
	logStashEvent["responseMessageIdentifier"] = response.ResponseMessageIdentifier
	xml, err := response.BuildXML()
	if err != nil {
		logStashEvent["responseXML"] = fmt.Sprintf("%v", err)
		return
	}
	logStashEvent["responseXML"] = xml
}
