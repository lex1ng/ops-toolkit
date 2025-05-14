package util

import "encoding/json"

func ToJSON(obj interface{}) string {
	if str, err := json.Marshal(obj); err == nil {
		return string(str)
	}

	return ""
}
func ToJSONIndent(obj interface{}) string {
	if str, err := json.MarshalIndent(obj, "", "	"); err == nil {
		return string(str)
	}
	return ""
}
