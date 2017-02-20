package api

import (
	"io/ioutil"
	"net/http"

	"github.com/af83/edwig/core"
)

var newControllerMap = map[string](func(*Server) ControllerInterface){
	"_referentials": NewReferentialController,
	"_time":         NewTimeController,
	"_status":       NewStatusController,
}

var newWithReferentialControllerMap = map[string](func(*core.Referential) ControllerInterface){
	"stop_areas":       NewStopAreaController,
	"partners":         NewPartnerController,
	"lines":            NewLineController,
	"stop_visits":      NewStopVisitController,
	"vehicle_journeys": NewVehicleJourneyController,
}

type RestfulRessource interface {
	Index(response http.ResponseWriter)
	Show(response http.ResponseWriter, identifier string)
	Delete(response http.ResponseWriter, identifier string)
	Update(response http.ResponseWriter, identifier string, body []byte)
	Create(response http.ResponseWriter, body []byte)
}

type ControllerInterface interface {
	serve(response http.ResponseWriter, request *http.Request, value string)
}

type Controller struct {
	restfulRessource RestfulRessource
}

func getRequestBody(response http.ResponseWriter, request *http.Request) []byte {
	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		http.Error(response, "Invalid request: Can't read request body", 400)
		return nil
	}
	if len(body) == 0 {
		http.Error(response, "Invalid request: Empty body", 400)
		return nil
	}
	return body
}

func (controller *Controller) serve(response http.ResponseWriter, request *http.Request, identifier string) {
	switch request.Method {
	case "GET":
		if identifier == "" {
			controller.restfulRessource.Index(response)
		} else {
			controller.restfulRessource.Show(response, identifier)
		}
	case "DELETE":
		if identifier == "" {
			http.Error(response, "Invalid request", 400)
			return
		}
		controller.restfulRessource.Delete(response, identifier)
	case "PUT":
		if identifier == "" {
			http.Error(response, "Invalid request", 400)
			return
		}
		body := getRequestBody(response, request)
		if body == nil {
			return
		}
		controller.restfulRessource.Update(response, identifier, body)
	case "POST":
		if identifier != "" {
			http.Error(response, "Invalid request", 400)
			return
		}
		body := getRequestBody(response, request)
		if body == nil {
			return
		}
		controller.restfulRessource.Create(response, body)
	}
}
