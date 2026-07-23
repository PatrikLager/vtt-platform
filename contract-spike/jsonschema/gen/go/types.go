// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    actor, err := UnmarshalActor(bytes)
//    bytes, err = actor.Marshal()
//
//    attackRolled, err := UnmarshalAttackRolled(bytes)
//    bytes, err = attackRolled.Marshal()
//
//    moveTokenRequest, err := UnmarshalMoveTokenRequest(bytes)
//    bytes, err = moveTokenRequest.Marshal()
//
//    tokenMoved, err := UnmarshalTokenMoved(bytes)
//    bytes, err = tokenMoved.Marshal()

package jsgen

import "encoding/json"

func UnmarshalActor(data []byte) (Actor, error) {
	var r Actor
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Actor) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAttackRolled(data []byte) (AttackRolled, error) {
	var r AttackRolled
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AttackRolled) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalMoveTokenRequest(data []byte) (MoveTokenRequest, error) {
	var r MoveTokenRequest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *MoveTokenRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalTokenMoved(data []byte) (TokenMoved, error) {
	var r TokenMoved
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *TokenMoved) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Actor struct {
	ActorID    string                 `json:"actorId"`
	Attributes map[string]int64       `json:"attributes"`
	ModuleData map[string]interface{} `json:"moduleData"`
	ModuleID   string                 `json:"moduleId"`
	Name       string                 `json:"name"`
	Resources  map[string]ActorSchema `json:"resources"`
}

type ActorSchema struct {
	Current int64 `json:"current"`
	Max     int64 `json:"max"`
}

type AttackRolled struct {
	AttackerID string            `json:"attackerId"`
	Expression string            `json:"expression"`
	Modifiers  []ModifierElement `json:"modifiers"`
	Outcome    string            `json:"outcome"`
	Rolls      []RollElement     `json:"rolls"`
	TargetID   string            `json:"targetId"`
	Total      int64             `json:"total"`
	Versus     string            `json:"versus"`
}

type ModifierElement struct {
	Source string `json:"source"`
	Value  int64  `json:"value"`
}

type RollElement struct {
	Die    int64 `json:"die"`
	Result int64 `json:"result"`
}

type MoveTokenRequest struct {
	To      To     `json:"to"`
	TokenID string `json:"tokenId"`
}

type To struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

type TokenMoved struct {
	From    From   `json:"from"`
	SceneID string `json:"sceneId"`
	To      From   `json:"to"`
	TokenID string `json:"tokenId"`
}

type From struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}
