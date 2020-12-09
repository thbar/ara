package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"bitbucket.org/enroute-mobi/ara/audit"
	"bitbucket.org/enroute-mobi/ara/clock"
	"bitbucket.org/enroute-mobi/ara/core"
	"bitbucket.org/enroute-mobi/ara/logger"
)

type SIRILiteVehicleMonitoringRequestHandler struct {
	requestUrl string
	filters    url.Values
}

func (handler *SIRILiteVehicleMonitoringRequestHandler) ConnectorType() string {
	return core.SIRI_LITE_VEHICLE_MONITORING_REQUEST_BROADCASTER
}

func (handler *SIRILiteVehicleMonitoringRequestHandler) Respond(connector core.Connector, rw http.ResponseWriter, message *audit.BigQueryMessage) {
	logger.Log.Debugf("Siri Lite VehicleMonitoring %s", handler.requestUrl)

	t := clock.DefaultClock().Now()

	response := connector.(core.VehicleMonitoringRequestBroadcaster).RequestVehicles(handler.requestUrl, handler.filters, message)

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		logger.Log.Debugf("Internal error while Marshaling a SiriLite response in vehicle monitoring handler: %v", err)
		return
	}
	n, err := rw.Write(jsonBytes)
	if err != nil {
		logger.Log.Debugf("Internal error while writing a SiriLite response in vehicle monitoring handler: %v", err)
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
	}

	message.Type = "VehicleMonitoringRequest"
	message.ResponseRawMessage = string(jsonBytes)
	message.ResponseSize = int64(n)
	message.ProcessingTime = time.Since(t).Seconds()
	audit.CurrentBigQuery().WriteEvent(message)
}
