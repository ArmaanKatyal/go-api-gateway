package main

type IPWhiteList struct {
	whitelist map[string]bool
}

func populateWhiteList(w *IPWhiteList, iplist []string) {
	if len(iplist) == 0 {
		// Allow all ip ranges, but this only works if whitelist is empty in the config
		w.whitelist["ALL"] = true
	} else {
		for _, ip := range iplist {
			w.whitelist[ip] = true
		}
	}
}

func NewIPWhiteList() *IPWhiteList {
	w := IPWhiteList{
		whitelist: make(map[string]bool),
	}
	return &w
}

func (w *IPWhiteList) Allowed(ip string) bool {
	if _, exists := w.whitelist["ALL"]; exists {
		return true
	}
	if _, found := w.whitelist[ip]; !found {
		return false
	}
	return true
}
