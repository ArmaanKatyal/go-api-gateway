package main

type IPWhiteList struct {
	Whitelist map[string]bool `json:"whitelist"`
}

func populateWhiteList(w *IPWhiteList, iplist []string) {
	if len(iplist) > 0 && iplist[0] == "ALL" {
		// Allow all ip ranges
		w.Whitelist["ALL"] = true
	} else {
		for _, ip := range iplist {
			w.Whitelist[ip] = true
		}
	}
}

func NewIPWhiteList() *IPWhiteList {
	w := IPWhiteList{
		Whitelist: make(map[string]bool),
	}
	return &w
}

func (w *IPWhiteList) Allowed(ip string) bool {
	if _, exists := w.Whitelist["ALL"]; exists {
		return true
	}
	if _, found := w.Whitelist[ip]; !found {
		return false
	}
	return true
}

func (w *IPWhiteList) GetWhitelist() map[string]bool {
	return w.Whitelist
}

func (w* IPWhiteList) UpdateWhitelist(newlist map[string]bool) {
	w.Whitelist = newlist
}