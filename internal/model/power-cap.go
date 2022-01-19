package model

import "github.com/google/uuid"

type PowerCapSnapshotParameter struct {
	Xnames []string `json:"xnames"`
}

type PowerCapPatchParameter struct {
	Components []Component `json:"components"`
}

type Component struct {
	Xname    string    `json:"xname"`
	Controls []Control `json:"controls"`
}

type Control struct {
	Name  string `json:"name"`
	Value int    `json:"value"` //TODO is this the right data type? can it be double?
}

type PowerCapTaskCreation struct {
	TaskID uuid.UUID `json:"taskID"`
}
