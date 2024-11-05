package definition

import (
	"github.com/zenon-network/go-zenon/common"
	"github.com/zenon-network/go-zenon/common/db"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/abi"
	"github.com/zenon-network/go-zenon/vm/constants"
	"strings"
)

const (
	jsonGovernance = `
	[
		{"type":"function","name":"ProposeAction", "inputs":[
			{"name":"name","type":"string"},
			{"name":"description","type":"string"},
			{"name":"url","type":"string"},
			{"name":"destination","type":"address"},
			{"name":"data","type":"string"},
		]},

		{"type":"function","name":"ExecuteAction", "inputs":[
			{"name":"id","type":"hash"}
		]},

		{"type":"variable","name":"action","inputs":[
			{"name":"id","type":"hash"},
			{"name":"owner","type":"address"},
			{"name":"name","type":"string"},
			{"name":"description","type":"string"},
			{"name":"url","type":"string"},
			{"name":"destination","type":"address"},
			{"name":"data","type":"string"},
			{"name":"creationTimestamp","type":"int64"},
			{"name":"executed","type":"bool"}
		]}
	]`

	ProposeActionMethodName = "ProposeAction"
	ExecuteActionMethodName = "ExecuteAction"

	actionVariableName = "action"
)

var (
	ABIGovernance = abi.JSONToABIContract(strings.NewReader(jsonGovernance))

	actionKeyPrefix uint8 = 0
)

type ActionParam struct {
	Id                types.Hash
	Owner             types.Address
	Name              string
	Description       string
	Url               string
	Destination       types.Address
	Data              string
	CreationTimestamp int64
	Executed          bool
}

func (action *ActionParam) Save(context db.DB) {
	common.DealWithErr(context.Put(action.Key(),
		ABIGovernance.PackVariablePanic(
			ProjectVariableName,
			action.Owner,
			action.Name,
			action.Description,
			action.Url,
			action.Destination,
			action.Data,
			action.CreationTimestamp,
			action.Executed,
		)))
}
func (action *ActionParam) Delete(context db.DB) {
	common.DealWithErr(context.Delete(action.Key()))
}
func (action *ActionParam) Key() []byte {
	return common.JoinBytes([]byte{actionKeyPrefix}, action.Id.Bytes())
}

func GetAction(context db.DB, id types.Hash) (*ActionParam, error) {
	key := (&ActionParam{Id: id}).Key()
	data, err := context.Get(key)
	common.DealWithErr(err)
	if len(data) == 0 {
		return nil, constants.ErrDataNonExistent
	} else {
		action := new(ActionParam)
		ABIGovernance.UnpackVariablePanic(action, actionVariableName, data)
		action.Id = types.BytesToHashPanic(key[1:33])
		return action, nil
	}
}
