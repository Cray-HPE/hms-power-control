package domain

import (
	"github.com/Cray-HPE/hms-power-control/internal/model"
	"github.com/google/uuid"
)

func SnapshotPowerCap(parameters model.PowerCapSnapshotParameter) (pb model.Passback) {
	//TODO stuff here!
	pb = model.BuildSuccessPassback(501, "SnapshotPowerCap")
	return pb
}

func PatchPowerCap(parameters model.PowerCapPatchParameter) (pb model.Passback) {
	//TODO stuff here!
	pb = model.BuildSuccessPassback(501, "PatchPowerCap")
	return pb
}

func GetPowerCap() (pb model.Passback) {
	//TODO stuff here!
	pb = model.BuildSuccessPassback(501, "GetPowerCap")
	return pb
}

func GetPowerCapQuery(taskID uuid.UUID) (pb model.Passback) {
	//TODO stuff here!
	pb = model.BuildSuccessPassback(501, "GetPowerCapQuery")
	return pb
}
