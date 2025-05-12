package nodes

import "fmt"

func CompareResource(have, want ResourceList) (bool, []string) {

	meet := true
	var notMeetResource []string
	for k, v := range want {
		reason := ""
		if h, ok := have[k]; !ok {
			meet = false
			reason = fmt.Sprintf("want %v: %s, have 0", v.Requests, k)
		} else {
			if h.Requests+v.Requests > h.Capacity {
				meet = false
				reason = fmt.Sprintf("want %v: %s, have %v", v.Requests, k, h.Capacity-h.Requests)
			}
		}
		if reason != "" {
			notMeetResource = append(notMeetResource, reason)
		}
	}

	return meet, notMeetResource

}
