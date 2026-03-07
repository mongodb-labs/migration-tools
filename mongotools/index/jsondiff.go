package index

import (
	"encoding/json"
	"fmt"

	"github.com/wI2L/jsondiff"
)

func describeJSONDiff(a, b []byte) (string, error) {
	patch, err := jsondiff.CompareJSON(
		a,
		b,
		jsondiff.Factorize(),
	)
	if err != nil {
		return "", fmt.Errorf("creating JSON patch: %w", err)
	}

	patchStr, err := fixJSONPatchFieldOrder([]byte(patch.String()))
	if err != nil {
		patchStr = fmt.Appendf(nil, "%s (failed to normalize order: %v)", patch.String(), err)
	}

	return string(patchStr), nil
}

// The jsondiff library’s patch.String() puts the fields in a weird order:
//
//	{"value":true,"op":"add","path":"/sparse"}
//
// This orders the fields more logically.
func fixJSONPatchFieldOrder(in []byte) ([]byte, error) {
	patchOrderStruct := struct {
		Op    string  `json:"op"`
		From  *string `json:"from,omitempty"`
		Path  string  `json:"path"`
		Value *any    `json:"value,omitempty"`
	}{}

	err := json.Unmarshal(in, &patchOrderStruct)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON for reordering: %w", err)
	}

	return json.Marshal(patchOrderStruct)
}
