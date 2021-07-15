package networkapi

import (
	"encoding/json"

	"github.com/pkg/errors"
)

func unmarshalIDs(data []byte) ([]int, error) {
	type idOnly struct {
		ID int `json:"id,omitempty"`
	}
	var ids []idOnly
	err := json.Unmarshal(data, &ids)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal ids from %q", string(data))
	}
	var result []int
	for _, id := range ids {
		result = append(result, id.ID)
	}
	return result, nil
}

func unmarshalField(data []byte, fieldName string, v interface{}) error {
	var result map[string]json.RawMessage
	err := json.Unmarshal(data, &result)
	if err != nil {
		return errors.Wrapf(err, "unable to unmarshal %q", string(data))
	}
	entry, ok := result[fieldName]
	if !ok {
		return errors.Errorf("field %q not found in result %s", fieldName, string(data))
	}
	return json.Unmarshal(entry, v)
}

func marshalField(fieldName string, obj interface{}) ([]byte, error) {
	newObj := map[string]interface{}{
		fieldName: obj,
	}
	return json.Marshal(newObj)
}
