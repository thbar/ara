package model

type TransactionalModel struct {
	parent Model

	stopAreas       *TransactionalStopAreas
	stopVisits      *TransactionalStopVisits
	vehicleJourneys *TransactionalVehicleJourneys
}

func NewTransactionalModel(parent Model) *TransactionalModel {
	model := &TransactionalModel{parent: parent}
	model.stopAreas = NewTransactionalStopAreas(parent)
	model.stopVisits = NewTransactionalStopVisits(parent)
	model.vehicleJourneys = NewTransactionalVehicleJourneys(parent)
	return model
}

func (model *TransactionalModel) StopAreas() StopAreas {
	return model.stopAreas
}

func (model *TransactionalModel) StopVisits() StopVisits {
	return model.stopVisits
}

func (model *TransactionalModel) VehicleJourneys() VehicleJourneys {
	return model.vehicleJourneys
}

func (model *TransactionalModel) NewTransaction() *Transaction {
	return NewTransaction(model)
}

func (model *TransactionalModel) Commit() error {
	if err := model.stopAreas.Commit(); err != nil {
		return err
	}
	if err := model.stopVisits.Commit(); err != nil {
		return err
	}
	if err := model.vehicleJourneys.Commit(); err != nil {
		return err
	}
	return nil
}

func (model *TransactionalModel) Rollback() error {
	if err := model.stopAreas.Rollback(); err != nil {
		return err
	}
	if err := model.stopVisits.Rollback(); err != nil {
		return err
	}
	if err := model.vehicleJourneys.Rollback(); err != nil {
		return err
	}
	return nil
}
